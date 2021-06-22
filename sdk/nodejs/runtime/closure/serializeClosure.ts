// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { log } from "../..";
import { Resource } from "../../resource";
import * as closure from "./createClosure";
import * as utils from "./utils";

/**
 * SerializeFunctionArgs are arguments used to serialize a JavaScript function
 */
export interface SerializeFunctionArgs {
    /**
     * The name to export from the module defined by the generated module text.  Defaults to 'handler'.
     */
    exportName?: string;
    /**
     * A function to prevent serialization of certain objects captured during the serialization.  Primarily used to
     * prevent potential cycles.
     */
    serialize?: (o: any) => boolean;
    /**
     * If this is a function which, when invoked, will produce the actual entrypoint function.
     * Useful for when serializing a function that has high startup cost that only wants to be
     * run once. The signature of this function should be:  () => (provider_handler_args...) => provider_result
     *
     * This will then be emitted as: `exports.[exportName] = serialized_func_name();`
     *
     * In other words, the function will be invoked (once) and the resulting inner function will
     * be what is exported.
     */
    isFactoryFunction?: boolean;
    /**
     * The resource to log any errors we encounter against.
     */
    logResource?: Resource;
    /**
     * If true, allow secrets to be serialized into the function. This should only be set to true if the calling
     * code will handle this and propoerly wrap the resulting text in a Secret before passing it into any Resources
     * or serializing it to any other output format. If set, the `containsSecrets` property on the returned
     * SerializedFunction object will indicate whether secrets were serialized into the function text.
     */
    allowSecrets?: boolean;
}

/**
 * SerializeFunction is a representation of a serialized JavaScript function.
 */
export interface SerializedFunction {
    /**
     * The text of a JavaScript module which exports a single name bound to an appropriate value.
     * In the case of a normal function, this value will just be serialized function.  In the case
     * of a factory function this value will be the result of invoking the factory function.
     */
    text: string;
    /**
     * The name of the exported module member.
     */
    exportName: string;
    /**
     * True if the serialized function text includes serialization of secret
     */
    containsSecrets: boolean;
}

/**
 * serializeFunction serializes a JavaScript function into a text form that can be loaded in another execution context,
 * for example as part of a function callback associated with an AWS Lambda.  The function serialization captures any
 * variables captured by the function body and serializes those values into the generated text along with the function
 * body.  This process is recursive, so that functions referenced by the body of the serialized function will themselves
 * be serialized as well.  This process also deeply serializes captured object values, including prototype chains and
 * property descriptors, such that the semantics of the function when deserialized should match the original function.
 *
 * There are several known limitations:
 * - If a native function is captured either directly or indirectly, closure serialization will return an error.
 * - Captured values will be serialized based on their values at the time that `serializeFunction` is called.  Mutations
 *   to these values after that (but before the deserialized function is used) will not be observed by the deserialized
 *   function.
 *
 * @param func The JavaScript function to serialize.
 * @param args Arguments to use to control the serialization of the JavaScript function.
 */
export async function serializeFunction(
        func: Function,
        args: SerializeFunctionArgs = {}): Promise<SerializedFunction> {

    const exportName = args.exportName || "handler";
    const serialize = args.serialize || (_ => true);
    const isFactoryFunction = args.isFactoryFunction === undefined ? false : args.isFactoryFunction;

    const closureInfo = await closure.createClosureInfoAsync(func, serialize, args.logResource);
    if (!args.allowSecrets && closureInfo.containsSecrets) {
        throw new Error("Secret outputs cannot be captured by a closure.");
    }
    return serializeJavaScriptText(closureInfo, exportName, isFactoryFunction);
}

/**
 * @deprecated Please use 'serializeFunction' instead.
 */
export async function serializeFunctionAsync(
        func: Function,
        serialize?: (o: any) => boolean): Promise<string> {
    log.warn("'function serializeFunctionAsync' is deprecated.  Please use 'serializeFunction' instead.");

    serialize = serialize || (_ => true);
    const closureInfo = await closure.createClosureInfoAsync(func, serialize, /*logResource:*/ undefined);
    if (closureInfo.containsSecrets) {
        throw new Error("Secret outputs cannot be captured by a closure.");
    }
    return serializeJavaScriptText(closureInfo, "handler", /*isFactoryFunction*/ false).text;
}

/**
 * serializeJavaScriptText converts a FunctionInfo object into a string representation of a Node.js module body which
 * exposes a single function `exports.handler` representing the serialized function.
 *
 * @param c The FunctionInfo to be serialized into a module string.
 */
function serializeJavaScriptText(
        outerClosure: closure.ClosureInfo,
        exportName: string,
        isFactoryFunction: boolean): SerializedFunction {

    // Now produce a textual representation of the closure and its serialized captured environment.

    // State used to build up the environment variables for all the funcs we generate.
    // In general, we try to create idiomatic code to make the generated code not too
    // hideous.  For example, we will try to generate code like:
    //
    //      var __e1 = [1, 2, 3] // or
    //      var __e2 = { a: 1, b: 2, c: 3 }
    //
    // However, for non-common cases (i.e. sparse arrays, objects with configured properties,
    // etc. etc.) we will spit things out in a much more verbose fashion that eschews
    // prettyness for correct semantics.
    const envEntryToEnvVar = new Map<closure.Entry, string>();
    const envVarNames = new Set<string>();
    const functionInfoToEnvVar = new Map<closure.FunctionInfo, string>();

    let environmentText = "";
    let functionText = "";

    const outerFunctionName = emitFunctionAndGetName(outerClosure.func);

    if (environmentText) {
        environmentText = "\n" + environmentText;
    }

    // Export the appropriate value.  For a normal function, this will just be exporting the name of
    // the module function we created by serializing it.  For a factory function this will export
    // the function produced by invoking the factory function once.
    let text: string;
    const exportText = `exports.${exportName} = ${outerFunctionName}${isFactoryFunction ? "()" : ""};`;
    if (isFactoryFunction) {
        // for a factory function, we need to call the function at the end.  That way all the logic
        // to set up the environment has run.
        text = environmentText + functionText + "\n" + exportText;
    }
    else {
        text = exportText + "\n" + environmentText + functionText;
    }

    return { text, exportName, containsSecrets: outerClosure.containsSecrets };

    function emitFunctionAndGetName(functionInfo: closure.FunctionInfo): string {
        // If this is the first time seeing this function, then actually emit the function code for
        // it.  Otherwise, just return the name of the emitted function for anyone that wants to
        // reference it from their own code.
        let functionName = functionInfoToEnvVar.get(functionInfo);
        if (!functionName) {
            functionName = functionInfo.name
                ? createEnvVarName(functionInfo.name, /*addIndexAtEnd:*/ false)
                : createEnvVarName("f", /*addIndexAtEnd:*/ true);
            functionInfoToEnvVar.set(functionInfo, functionName);

            emitFunctionWorker(functionInfo, functionName);
        }

        return functionName;
    }

    function emitFunctionWorker(functionInfo: closure.FunctionInfo, varName: string) {
        const capturedValues = envFromEnvObj(functionInfo.capturedValues);

        const thisCapture = capturedValues.this;
        const argumentsCapture = capturedValues.arguments;

        delete capturedValues.this;
        delete capturedValues.arguments;

        const parameters = [...Array(functionInfo.paramCount)].map((_, index) => `__${index}`).join(", ");

        functionText += "\n" +
            "function " + varName + "(" + parameters + ") {\n" +
            "  return (function() {\n" +
            "    with(" + envObjToString(capturedValues) + ") {\n\n" +
            "return " + functionInfo.code + ";\n\n" +
            "    }\n" +
            "  }).apply(" + thisCapture + ", " + argumentsCapture + ").apply(this, arguments);\n" +
            "}\n";

        // If this function is complex (i.e. non-default __proto__, or has properties, etc.)
        // then emit those as well.
        emitComplexObjectProperties(varName, varName, functionInfo);

        if (functionInfo.proto !== undefined) {
            const protoVar = envEntryToString(functionInfo.proto, `${varName}_proto`);
            environmentText += `Object.setPrototypeOf(${varName}, ${protoVar});\n`;
        }
    }

    function envFromEnvObj(env: closure.PropertyMap): Record<string, string> {
        const envObj: Record<string, string> = {};
        for (const [keyEntry, { entry: valEntry }] of env) {
            if (typeof keyEntry.json !== "string") {
                throw new Error("PropertyMap key was not a string.");
            }

            const key = keyEntry.json;
            const val = envEntryToString(valEntry, key);
            envObj[key] = val;
        }
        return envObj;
    }

    function envEntryToString(envEntry: closure.Entry, varName: string): string {
        const envVar = envEntryToEnvVar.get(envEntry);
        if (envVar !== undefined) {
            return envVar;
        }

        // Complex objects may also be referenced from multiple functions.  As such, we have to
        // create variables for them in the environment so that all references to them unify to the
        // same reference to the env variable.  Effectively, we need to do this for any object that
        // could be compared for reference-identity.  Basic types (strings, numbers, etc.) have
        // value semantics and this can be emitted directly into the code where they are used as
        // there is no way to observe that you are getting a different copy.
        if (isObjOrArrayOrRegExp(envEntry)) {
            return complexEnvEntryToString(envEntry, varName);
        }
        else {
            // Other values (like strings, bools, etc.) can just be emitted inline.
            return simpleEnvEntryToString(envEntry, varName);
        }
    }

    function simpleEnvEntryToString(
            envEntry: closure.Entry, varName: string): string {

        if (envEntry.hasOwnProperty("json")) {
            return JSON.stringify(envEntry.json);
        }
        else if (envEntry.function !== undefined) {
            return emitFunctionAndGetName(envEntry.function);
        }
        else if (envEntry.module !== undefined) {
            return `require("${envEntry.module}")`;
        }
        else if (envEntry.output !== undefined) {
            return envEntryToString(envEntry.output, varName);
        }
        else if (envEntry.expr) {
            // Entry specifies exactly how it should be emitted.  So just use whatever
            // it wanted.
            return envEntry.expr;
        }
        else if (envEntry.promise) {
            return `Promise.resolve(${envEntryToString(envEntry.promise, varName)})`;
        }
        else {
            throw new Error("Malformed: " + JSON.stringify(envEntry));
        }
    }

    function complexEnvEntryToString(
            envEntry: closure.Entry, varName: string): string {
        // Call all environment variables __e<num> to make them unique.  But suffix
        // them with the original name of the property to help provide context when
        // looking at the source.
        const envVar = createEnvVarName(varName, /*addIndexAtEnd:*/ false);
        envEntryToEnvVar.set(envEntry, envVar);

        if (envEntry.object) {
            emitObject(envVar, envEntry.object, varName);
        }
        else if (envEntry.array) {
            emitArray(envVar, envEntry.array, varName);
        }
        else if (envEntry.regexp) {
            const { source, flags } = envEntry.regexp;
            const regexVal = `new RegExp(${JSON.stringify(source)}, ${JSON.stringify(flags)})`;
            const entryString = `var ${envVar} = ${regexVal};\n`;

            environmentText += entryString;
        }

        return envVar;
    }

    function createEnvVarName(baseName: string, addIndexAtEnd: boolean): string {
        const trimLeadingUnderscoreRegex = /^_*/g;
        const legalName = makeLegalJSName(baseName).replace(trimLeadingUnderscoreRegex, "");
        let index = 0;

        let currentName = addIndexAtEnd
            ? "__" + legalName + index
            : "__" + legalName;
        while (envVarNames.has(currentName)) {
            currentName = addIndexAtEnd
                ? "__" + legalName + index
                : "__" + index + "_" + legalName;
            index++;
        }

        envVarNames.add(currentName);
        return currentName;
    }

    function emitObject(envVar: string, obj: closure.ObjectInfo, varName: string): void {
        const complex = isComplex(obj);

        if (complex) {
            // we have a complex child.  Because of the possibility of recursion in
            // the object graph, we have to spit out this variable uninitialized first.
            // Then we can walk our children, creating a single assignment per child.
            // This way, if the child ends up referencing us, we'll have already emitted
            // the **initialized** variable for them to reference.
            if (obj.proto) {
                const protoVar = envEntryToString(obj.proto, `${varName}_proto`);
                environmentText += `var ${envVar} = Object.create(${protoVar});\n`;
            }
            else {
                environmentText += `var ${envVar} = {};\n`;
            }

            emitComplexObjectProperties(envVar, varName, obj);
        }
        else {
            // All values inside this obj are simple.  We can just emit the object
            // directly as an object literal with all children embedded in the literal.
            const props: string[] = [];

            for (const [keyEntry, { entry: valEntry }] of obj.env) {
                const keyName = typeof keyEntry.json === "string" ? keyEntry.json : "sym";
                const propName = envEntryToString(keyEntry, keyName);
                const propVal = simpleEnvEntryToString(valEntry, keyName);

                if (typeof keyEntry.json === "string" && utils.isLegalMemberName(keyEntry.json)) {
                    props.push(`${keyEntry.json}: ${propVal}`);
                }
                else {
                    props.push(`[${propName}]: ${propVal}`);
                }
            }

            const allProps = props.join(", ");
            const entryString = `var ${envVar} = {${allProps}};\n`;
            environmentText += entryString;
        }

        function isComplex(o: closure.ObjectInfo) {
            if (obj.proto !== undefined) {
                return true;
            }

            for (const v of o.env.values()) {
                if (entryIsComplex(v)) {
                    return true;
                }
            }

            return false;
        }

        function entryIsComplex(v: closure.PropertyInfoAndValue) {
            return !isSimplePropertyInfo(v.info) || deepContainsObjOrArrayOrRegExp(v.entry);
        }
    }

    function isSimplePropertyInfo(info: closure.PropertyInfo | undefined): boolean {
        if (!info) {
            return true;
        }

        return info.enumerable === true &&
               info.writable === true &&
               info.configurable === true &&
               !info.get && !info.set;
    }

    function emitComplexObjectProperties(
            envVar: string, varName: string, objEntry: closure.ObjectInfo): void {

        for (const [keyEntry, { info, entry: valEntry }] of objEntry.env) {
            const subName = typeof keyEntry.json === "string" ? keyEntry.json : "sym";
            const keyString = envEntryToString(keyEntry, varName + "_" + subName);
            const valString = envEntryToString(valEntry, varName + "_" + subName);

            if (isSimplePropertyInfo(info)) {
                // normal property.  Just emit simply as a direct assignment.
                if (typeof keyEntry.json === "string" && utils.isLegalMemberName(keyEntry.json)) {
                    environmentText += `${envVar}.${keyEntry.json} = ${valString};\n`;
                }
                else {
                    environmentText += `${envVar}${`[${keyString}]`} = ${valString};\n`;
                }
            }
            else {
                // complex property.  emit as Object.defineProperty
                emitDefineProperty(info!, valString, keyString);
            }
        }

        function emitDefineProperty(
            desc: closure.PropertyInfo, entryValue: string, propName: string) {

            const copy: any = {};
            if (desc.configurable) {
                copy.configurable = desc.configurable;
            }
            if (desc.enumerable) {
                copy.enumerable = desc.enumerable;
            }
            if (desc.writable) {
                copy.writable = desc.writable;
            }
            if (desc.get) {
                copy.get = envEntryToString(desc.get, `${varName}_get`);
            }
            if (desc.set) {
                copy.set = envEntryToString(desc.set, `${varName}_set`);
            }
            if (desc.hasValue) {
                copy.value = entryValue;
            }
            const line = `Object.defineProperty(${envVar}, ${propName}, ${ envObjToString(copy) });\n`;
            environmentText += line;
        }
    }

    function emitArray(
            envVar: string, arr: closure.Entry[], varName: string): void {
        if (arr.some(deepContainsObjOrArrayOrRegExp) || isSparse(arr) || hasNonNumericIndices(arr)) {
            // we have a complex child.  Because of the possibility of recursion in the object
            // graph, we have to spit out this variable initialized (but empty) first. Then we can
            // walk our children, knowing we'll be able to find this variable if they reference it.
            environmentText += `var ${envVar} = [];\n`;

            // Walk the names of the array properties directly. This ensures we work efficiently
            // with sparse arrays.  i.e. if the array has length 1k, but only has one value in it
            // set, we can just set htat value, instead of setting 999 undefineds.
            let length = 0;
            for (const key of Object.getOwnPropertyNames(arr)) {
                if (key !== "length") {
                    const entryString = envEntryToString(arr[<any>key], `${varName}_${key}`);
                    environmentText += `${envVar}${
                        isNumeric(key) ? `[${key}]` : `.${key}`} = ${entryString};\n`;
                    length++;
                }
            }
        }
        else {
            // All values inside this array are simple.  We can just emit the array elements in
            // place.  i.e. we can emit as ``var arr = [1, 2, 3]`` as that's far more preferred than
            // having four individual statements to do the same.
            const strings: string[] = [];
            for (let i = 0, n = arr.length; i < n; i++) {
                strings.push(simpleEnvEntryToString(arr[i], `${varName}_${i}`));
            }

            const entryString = `var ${envVar} = [${strings.join(", ")}];\n`;
            environmentText += entryString;
        }
    }
}
(<any>serializeJavaScriptText).doNotCapture = true;

const makeLegalRegex = /[^0-9a-zA-Z_]/g;
function makeLegalJSName(n: string) {
    return n.replace(makeLegalRegex, x => "");
}

function isSparse<T>(arr: Array<T>) {
    // getOwnPropertyNames for an array returns all the indices as well as 'length'.
    // so we subtract one to get all the real indices.  If that's not the same as
    // the array length, then we must have missing properties and are thus sparse.
    return arr.length !== (Object.getOwnPropertyNames(arr).length - 1);
}

function hasNonNumericIndices<T>(arr: Array<T>) {
    return Object.keys(arr).some(k => k !== "length" && !isNumeric(k));
}

function isNumeric(n: string) {
    return !isNaN(parseFloat(n)) && isFinite(+n);
}

function isObjOrArrayOrRegExp(env: closure.Entry): boolean {
    return env.object !== undefined || env.array !== undefined || env.regexp !== undefined;
}

function deepContainsObjOrArrayOrRegExp(env: closure.Entry): boolean {
    return isObjOrArrayOrRegExp(env) ||
        (env.output !== undefined && deepContainsObjOrArrayOrRegExp(env.output)) ||
        (env.promise !== undefined && deepContainsObjOrArrayOrRegExp(env.promise));
}

/**
 * Converts an environment object into a string which can be embedded into a serialized function
 * body.  Note that this is not JSON serialization, as we may have property values which are
 * variable references to other global functions. In other words, there can be free variables in the
 * resulting object literal.
 *
 * @param envObj The environment object to convert to a string.
 */
function envObjToString(envObj: Record<string, string>): string {
    return `{ ${Object.keys(envObj).map(k => `${k}: ${envObj[k]}`).join(", ")} }`;
}
