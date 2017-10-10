// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as acorn from "acorn";
import * as crypto from "crypto";
import * as estree from "estree";
import { relative as pathRelative } from "path";
import { debuggablePromise } from "./debuggable";
import * as log from "../log";

const acornwalk = require("acorn/dist/walk");
const nativeruntime = require("./native/build/Release/nativeruntime.node");

/**
 * Closure represents the serialized form of a JavaScript serverless function.
 */
export interface Closure {
    code: string;             // a serialization of the function's source code as text.
    runtime: string;          // the language runtime required to execute the serialized code.
    environment: Environment; // the captured lexical environment of variables to values, if any.
}

/**
 * Environment is the captured lexical environment for a closure.
 */
export type Environment = {[key: string]: EnvironmentEntry};

/**
 * EnvironmentEntry is the environment slot for a named lexically captured variable.
 */
export interface EnvironmentEntry {
    json?: any;               // a value which can be safely json serialized.
    closure?: Closure;        // a closure we are dependent on.
    obj?: Environment;        // an object which may contain nested closures.
    arr?: EnvironmentEntry[]; // an array which may contain nested closures.
    module?: string;          // a reference to a requirable module name.
}

/**
 * serializeClosure serializes a function and its closure environment into a form that is amenable to persistence
 * as simple JSON.  Like toString, it includes the full text of the function's source code, suitable for execution.
 * Unlike toString, it actually includes information about the captured environment.
 */
export function serializeClosure(func: Function): Promise<Closure> {
    // First get the async version.  We will then await it to turn it into a flattened, async-free computed closure.
    // This must be done "at the top" because we must not block the creation of the dataflow graph of closure
    // elements, since there may be cycles that can only resolve by creating the entire graph first.
    let closure: AsyncClosure = serializeClosureAsync(func);

    // Now turn the AsyncClosure into a normal closure, and return it.
    let flatCache = new Map<Promise<AsyncEnvironmentEntry>, EnvironmentEntry>();
    return flattenClosure(closure, flatCache);
}

async function flattenClosure(closure: AsyncClosure,
    flatCache: Map<Promise<AsyncEnvironmentEntry>, EnvironmentEntry>): Promise<Closure> {
    return {
        code: closure.code,
        runtime: closure.runtime,
        environment: await flattenEnvironment(closure.environment, flatCache),
    };
}

async function flattenEnvironment(env: AsyncEnvironment,
    flatCache: Map<Promise<AsyncEnvironmentEntry>, EnvironmentEntry>): Promise<Environment> {
    let result: Environment = {};
    for (let key of Object.keys(env)) {
        result[key] = await flattenEnvironmentEntry(env[key], flatCache);
    }
    return result;
}

async function flattenEnvironmentEntry(entry: Promise<AsyncEnvironmentEntry>,
    flatCache: Map<Promise<AsyncEnvironmentEntry>, EnvironmentEntry>): Promise<EnvironmentEntry> {
    // See if there's an entry for this object already; if there is, use it.
    let result: EnvironmentEntry | undefined = flatCache.get(entry);
    if (result) {
        return result;
    }

    // Otherwise, we need to create a new one, add it to the cache before recursing, and then go.  Note that we
    // DO NOT add a promise for the entry!  We add the entry object itself, to avoid deadlocks in which mutually
    // recursive functions end up trying to resolve the same entry on the same callstack.
    result = {};
    flatCache.set(entry, result);

    let e: AsyncEnvironmentEntry = await entry;
    if (e.hasOwnProperty("json")) {
        result.json = e.json;
    }
    else if (e.module) {
        result.module = e.module;
    }
    else if (e.closure) {
        result.closure = await flattenClosure(e.closure, flatCache);
    }
    else if (e.obj) {
        result.obj = await flattenEnvironment(e.obj, flatCache);
    }
    else if (e.arr) {
        let arr: EnvironmentEntry[] = [];
        for (let elem of e.arr) {
            arr.push(await flattenEnvironmentEntry(elem, flatCache));
        }
        result.arr = arr;
    }
    else {
        throw new Error(`Malformed flattened environment entry: ${e}`);
    }
    return result;
}

/**
 * AsyncClosure represents the eventual serialized form of a JavaScript serverless function.
 */
export interface AsyncClosure {
    code: string;                     // a serialization of the function's source code as text.
    runtime: string;                  // the language runtime required to execute the serialized code.
    environment: AsyncEnvironment; // the captured lexical environment of variables to values, if any.
}

/**
 * AsyncEnvironment is the eventual captured lexical environment for a closure.
 */
export type AsyncEnvironment = {[key: string]: Promise<AsyncEnvironmentEntry>};

/**
 * AsyncEnvironmentEntry is the eventual environment slot for a named lexically captured variable.
 */
export interface AsyncEnvironmentEntry {
    json?: any;                             // a value which can be safely json serialized.
    closure?: AsyncClosure;                 // a closure we are dependent on.
    obj?: AsyncEnvironment;                 // an object which may contain nested closures.
    arr?: Promise<AsyncEnvironmentEntry>[]; // an array which may contain nested closures.
    module?: string;                        // a reference to a requirable module name.
}

/**
 * entryCache stores a map of entry to promise, to support mutually recursive captures.
 */
let entryCache = new Map<Object, Promise<AsyncEnvironmentEntry>>();

/**
 * serializeClosureAsync does the work to create an asynchronous dataflow graph that resolves to a final closure.
 */
function serializeClosureAsync(func: Function): AsyncClosure {
    // Invoke the native runtime.  Note that we pass a callback to our function below to compute free variables.
    // This must be a callback and not the result of this function alone, since we may recursively compute them.
    //
    // N.B.  We use the Acorn parser to compute them.  This has the downside that we now have two parsers in the game,
    // V8 and Acorn (three if you include optional TypeScript), but has the significant advantage that V8's parser
    // isn't designed to be stable for 3rd party consumtpion.  Hence it would be brittle and a maintenance challenge.
    // This approach also avoids needing to write a big hunk of complex code in C++, which is nice.
    return <AsyncClosure>nativeruntime.serializeClosure(func, computeFreeVariables, serializeCapturedObject);
}

/**
 * serializeCapturedObject serializes an object, deeply, into something appropriate for an environment entry.
 */
function serializeCapturedObject(obj: any): Promise<AsyncEnvironmentEntry> {
    // See if we have a cache hit.  If yes, use the object as-is.
    let result: Promise<AsyncEnvironmentEntry> | undefined = entryCache.get(obj);
    if (result) {
        return result;
    }

    // If it doesn't exist, actually do it, but stick the promise in the cache first for recursive scenarios.
    let resultResolve: ((v: AsyncEnvironmentEntry) => void) | undefined = undefined;
    result = debuggablePromise(new Promise<AsyncEnvironmentEntry>((resolve) => { resultResolve = resolve; }));
    entryCache.set(obj, result);
    serializeCapturedObjectAsync(obj, resultResolve!);
    return result;
}

/**
 * serializeCapturedObjectAsync is the work-horse that actually performs object serialization.
 */
function serializeCapturedObjectAsync(obj: any, resolve: (v: AsyncEnvironmentEntry) => void): void {
    let moduleName = findRequirableModuleName(obj);
    if (obj === undefined || obj === null ||
            typeof obj === "boolean" || typeof obj === "number" || typeof obj === "string") {
        // Serialize primitives as-is.
        resolve({ json: obj });
    }
    else if (moduleName) {
        // Serialize any value which was found as a requirable module name as a reference to the module
        resolve({module: moduleName});
    }
    else if (obj instanceof Array) {
        // Recursively serialize elements of an array.
        let arr: Promise<AsyncEnvironmentEntry>[] = [];
        for (let elem of obj) {
            arr.push(serializeCapturedObject(elem));
        }
        resolve({ arr: arr });
    }
    else if (obj instanceof Function) {
        // Serialize functions recursively, and store them in a closure property.
        resolve({ closure: serializeClosureAsync(obj) });
    }
    else if (obj instanceof Promise) {
        // If this is a promise, we will await it and serialize the result instead.
        obj.then((v) => serializeCapturedObjectAsync(v, resolve));
    }
    else {
        // For all other objects, serialize all of their enumerable properties (skipping non-enumerable members, etc).
        let env: AsyncEnvironment = {};
        for (let key of Object.keys(obj)) {
            env[key] = serializeCapturedObject(obj[key]);
        }
        resolve({ obj: env });
    }
}

// These modules are built-in to Node.js, and are available via `require(...)`
// but are not stored in the `require.cache`.  They are guaranteed to be
// available at the unqualified names listed below. _Note_: This list is derived
// based on Node.js 6.x tree at: https://github.com/nodejs/node/tree/v6.x/lib
let builtInModuleNames = [
    "assert", "buffer", "child_process", "cluster", "console", "constants", "crypto",
    "dgram", "dns", "domain", "events", "fs", "http", "https", "module", "net", "os",
    "path", "process", "punycode", "querystring", "readline", "repl", "stream", "string_decoder",
    /* "sys" deprecated ,*/ "timers", "tls", "tty", "url", "util", "v8", "vm", "zlib",
];
let builtInModules = new Map<any, string>();
for (let name of builtInModuleNames) {
    builtInModules.set(require(name), name);
}

// findRequirableModuleName attempts to find a global name bound to the object, which can
// be used as a stable reference across serialization.
function findRequirableModuleName(obj: any): string | undefined  {
    // First, check the built-in modules
    let key = builtInModules.get(obj);
    if (key) {
        return key;
    }
    // Next, check the Node module require cache, which will store cached values
    // of all non-built-in Node modules loaded by the program so far. _Note_: We
    // don't pre-compute this because the require cache will get populated
    // dynamically during execution.
    for (let path of Object.keys(require.cache)) {
        if (require.cache[path].exports === obj) {
            // Rewrite the path to be a local module reference relative to the
            // current working directory
            let modPath = pathRelative(process.cwd(), path);
            return "./" + modPath;
        }
    }

    // Else, return that no global name is available for this object.
    return undefined;
}

/**
 * computeFreeVariables computes the set of free variables in a given function string.  Note that this string is
 * expected to be the usual V8-serialized function expression text.
 */
function computeFreeVariables(funcstr: string): string[] {
    log.debug(`Computing free variables for function: ${funcstr}`);
    if (funcstr.indexOf("[native code]") !== -1) {
        throw new Error(`Cannot serialize native code function: "${funcstr}"`);
    }

    let opts: acorn.Options = {
        ecmaVersion: 8,
        sourceType: "script",
    };
    let parser = new acorn.Parser(opts, funcstr);
    let program: estree.Program;
    try {
        program = parser.parse();
    }
    catch (err) {
        throw new Error(`Could not parse function: ${err}\n${funcstr}`);
    }

    // Now that we've parsed the program, compute the free variables, and return them.
    return new FreeVariableComputer().compute(program);
}

type walkCallback = (node: estree.BaseNode, state: any) => void;

const nodeModuleGlobals: {[key: string]: boolean} = {
    "__dirname": true,
    "__filename": true,
    "exports": true,
    "module": true,
    "require": true,
};

class FreeVariableComputer {
    private frees: {[key: string]: boolean};   // the in-progress list of free variables.
    private scope: {[key: string]: boolean}[]; // a chain of current scopes and variables.
    private functionVars: string[];            // list of function-scoped variables (vars).

    private static isBuiltIn(ident: string): boolean {
        // Anything in the global dictionary is a built-in.  So is anything that's a global Node.js object;
        // note that these only exist in the scope of modules, and so are not truly global in the usual sense.
        // See https://nodejs.org/api/globals.html for more details.
        return global.hasOwnProperty(ident) || nodeModuleGlobals[ident];
    }

    public compute(program: estree.Program): string[] {
        // Reset the state.
        this.frees = {};
        this.scope = [];
        this.functionVars = [];

        // Recurse through the tree.
        acornwalk.recursive(program, {}, {
            Identifier: this.visitIdentifier.bind(this),
            ThisExpression: this.visitThisExpression.bind(this),
            BlockStatement: this.visitBlockStatement.bind(this),
            CatchClause: this.visitCatchClause.bind(this),
            CallExpression: this.visitCallExpression.bind(this),
            FunctionDeclaration: this.visitFunctionDeclaration.bind(this),
            FunctionExpression: this.visitBaseFunction.bind(this),
            ArrowFunctionExpression: this.visitBaseFunction.bind(this),
            VariableDeclaration: this.visitVariableDeclaration.bind(this),
        });

        // Now just return all variables whose value is true.  Filter out any that are part of the built-in
        // Node.js global object, however, since those are implicitly availble on the other side of serialization.
        let freeVars: string[] = [];
        for (let key of Object.keys(this.frees)) {
            if (this.frees[key] && !FreeVariableComputer.isBuiltIn(key)) {
                freeVars.push(key);
            }
        }
        return freeVars;
    }

    private visitIdentifier(node: estree.Identifier, state: any, cb: walkCallback): void {
        // Remember undeclared identifiers during the walk, as they are possibly free.
        let name: string = node.name;
        for (let i = this.scope.length - 1; i >= 0; i--) {
            if (this.scope[i][name]) {
                // This is currently known in the scope chain, so do not add it as free.
                break;
            } else if (i === 0) {
                // We reached the top of the scope chain and this wasn't found; it's free.
                this.frees[name] = true;
            }
        }
    }

    private visitThisExpression(node: estree.Identifier, state: any, cb: walkCallback): void {
        // Mark references to the built-in 'this' variable as free.
        this.frees["this"] = true;
    }

    private visitBlockStatement(node: estree.BlockStatement, state: any, cb: walkCallback): void {
        // Push new scope, visit all block statements, and then restore the scope.
        this.scope.push({});
        for (let stmt of node.body) {
            cb(stmt, state);
        }
        this.scope.pop();
    }

    private visitFunctionDeclaration(node: estree.FunctionDeclaration, state: any, cb: walkCallback): void {
        // A function declaration is special in one way: its identifier is added to the current function's
        // var-style variables, so that its name is in scope no matter the order of surrounding references to it.
        this.functionVars.push(node.id.name);
        this.visitBaseFunction(node, state, cb);
    }

    private visitBaseFunction(node: estree.BaseFunction, state: any, cb: walkCallback): void {
        // First, push new free vars list, scope, and function vars
        let oldFrees: {[key: string]: boolean} = this.frees;
        let oldFunctionVars: string[] = this.functionVars;
        this.frees = {};
        this.functionVars = [];
        this.scope.push({});

        // Add all parameters to the scope.  By visiting the parameters, they end up being seen as
        // identifiers, and therefore added to the free variables list.  We then migrate them to the scope.
        for (let param of node.params) {
            cb(param, state);
        }
        for (let param of Object.keys(this.frees)) {
            if (this.frees[param]) {
                this.scope[this.scope.length-1][param] = true;
            }
        }
        this.frees = {};

        // Next, visit the body underneath this new context.
        cb(node.body, state);

        // Remove any function-scoped variables that we encountered during the walk.
        for (let v of this.functionVars) {
            this.frees[v] = false;
        }

        // If the function is not an arrow, then its `this` is also a
        // function-scoped variable and should be removed.
        if ((node as estree.ArrowFunctionExpression).type !== "ArrowFunctionExpression") {
            this.frees["this"] = false;
        }

        // Restore the prior context and merge our free list with the previous one.
        this.scope.pop();
        this.functionVars = oldFunctionVars;
        for (let free of Object.keys(this.frees)) {
            if (this.frees[free]) {
                oldFrees[free] = true;
            }
        }
        this.frees = oldFrees;
    }

    private visitCatchClause(node: estree.CatchClause, state: any, cb: walkCallback): void {
        // Add the catch pattern to the scope as a variable.
        let oldFrees: {[key: string]: boolean} = this.frees;
        this.frees = {};
        this.scope.push({});
        cb(node.param, state);
        for (let param of Object.keys(this.frees)) {
            if (this.frees[param]) {
                this.scope[this.scope.length-1][param] = true;
            }
        }
        this.frees = oldFrees;

        // And then visit the block without adding them as free variables.
        cb(node.body, state);

        // Relinquish the scope so the error patterns aren't available beyond the catch.
        this.scope.pop();
    }

    private visitCallExpression(node: estree.SimpleCallExpression, state: any, cb: walkCallback): void {
        // Most call expressions are normal.  But we must special case one kind of function:
        // TypeScript's __awaiter functions.  They are of the form `__awaiter(this, void 0, void 0, function* (){})`,
        // which will cause us to attempt to capture and serialize the entire surrounding function in
        // which any lambda is created (thanks to `this`).  That spirals into craziness, and bottoms out on native
        // functions which we cannot serialize.  We only want to capture `this` if the user code mentioned it.
        cb(node.callee, state);
        let isAwaiterCall: boolean =
            (node.callee.type === "Identifier" && (node.callee as estree.Identifier).name === "__awaiter");
        for (let i = 0; i < node.arguments.length; i++) {
            if (i > 0 || !isAwaiterCall) {
                cb(node.arguments[i], state);
            }
        }
    }

    private visitVariableDeclaration(node: estree.VariableDeclaration, state: any, cb: walkCallback): void {
        for (let decl of node.declarations) {
            // Walk the declaration's `id` property (which may be an Identifier
            // or Pattern) using a fresh AST walker which will capture any
            // variables declared by this variable declaration. Pass the
            // variable kind in as state for the walker so that variables can be
            // registered into the right scope (function-wide for var and
            // scope-wide for let/const).
            acornwalk.recursive(decl.id, {varKind: node.kind}, {
                Identifier: this.visitVariableDeclarationIdentifier.bind(this),
                ObjectPattern: this.visitVariableDeclarationObjectPattern.bind(this),
                ArrayPattern: this.visitVariableDeclarationArrayPattern.bind(this),
                AssignmentPattern: this.visitVariableDeclarationAssignmentPattern.bind(this),
                RestElement: this.visitVariableDeclarationRestPattern.bind(this),
            });

            // Make sure to walk the initializer.
            if (decl.init) {
                cb(decl.init, state);
            }
        }
    }

    private visitVariableDeclarationIdentifier(node: estree.Identifier, state: any, cb: walkCallback): void {
        // If the declaration is an identifier, it isn't a free variable, for whatever scope it
        // pertains to (function-wide for var and scope-wide for let/const).  Track it so we can
        // remove any subseqeunt references to that variable, so we know it isn't free.
        if (state.varKind === "var") {
            this.functionVars.push(node.name);
        } else {
            this.scope[this.scope.length-1][node.name] = true;
        }
    }

    private visitVariableDeclarationObjectPattern(node: estree.ObjectPattern, state: any, cb: walkCallback): void {
        for (let prop of node.properties) {
            // Recurse into each `value` pattern, which may contain nested variable bindings.
            cb(prop.value, state);
        }
    }

    private visitVariableDeclarationArrayPattern(node: estree.ArrayPattern, state: any, cb: walkCallback): void {
        for (let pattern of node.elements) {
            // Recurse into each nested pattern, which may contain nested
            // variable bindings.  Skip gaps in the array pattern.
            if (pattern) {
                cb(pattern, state);
            }
        }
    }

    private visitVariableDeclarationAssignmentPattern(node: estree.AssignmentPattern, state: any, cb: walkCallback)
    : void {
        // Recurse into the left side of this assignment pattern.
        cb(node.left, state);
    }

    private visitVariableDeclarationRestPattern(node: estree.RestElement, state: any, cb: walkCallback): void {
        // Recurse into the argument pattern of the rest pattern.
        cb(node.argument, state);
    }
}

/**
 * serializeJavaScript Text converts a Closure object into a string
 * representation of a Node.js module body which exposes a single function
 * `exports.handler` representing the serialized function.
 * @param c The Closure to be serialized into a module string.
 */
export function serializeJavaScriptText(c: Closure): string {
    // Ensure the closure is targeting a supported runtime.
    if (c.runtime !== "nodejs") {
        throw new Error(`Runtime '${c.runtime}' not yet supported (currently only 'nodejs')`);
    }

    // Now produce a textual representation of the closure and its serialized captured environment.
    let funcsForClosure = new FuncsForClosure(c);
    let funcs = funcsForClosure.funcs;
    let text = "exports.handler = " + funcsForClosure.root + ";\n\n";
    for (let name of Object.keys(funcs)) {
        text +=
            "function " + name + "() {\n" +
            "  var _this;\n" +
            "  with(" + envObjToString(funcs[name].env) + ") {\n" +
            "    return (function() {\n\n" +
            "return " + funcs[name].code + "\n\n" +
            "    }).apply(_this).apply(this, arguments);\n" +
            "  }\n" +
            "}\n" +
            "\n";
    }
    return text;
}

export function getClosureHash_forTestingPurposes(closure: Closure): string {
    return new FuncsForClosure(closure).root;
}

interface FuncEnv {
    code: string;
    env: { [key: string]: string; };
}

/**
 * FuncsForClosure collects all the function defintions needed to support serialization of a given Closure object.
 * Context is the shape of the context object passed to a Function callback.
 * Note that a Closure object can reference other Closure objects and can also have cycles, so we recursively walk the
 * graph and cache serialized nodes along the way to avoid cycles.
 */
class FuncsForClosure {
    public funcs: { [hash: string]: FuncEnv }; // a cache of functions.
    public root: string;                       // the root closure hash.

    constructor(closure: Closure) {
        this.funcs = {};
        this.root = this.createFuncForClosure(closure);
    }

    private createFuncForClosure(closure: Closure): string {
        // Produce a hash to identify the function.
        let hash = this.createFunctionHash(closure);

        // Now only store if this function hasn't already been hashed.
        if (this.funcs[hash] === undefined) {
            this.funcs[hash] = {
                code: closure.code,
                env: {}, // initialize as empty - update after recursive call
            };
            this.funcs[hash].env = this.envFromEnvObj(closure.environment);
        }

        return hash;
    }

    private createFunctionHash(closure: Closure): string {
        let shasum = crypto.createHash("sha1");

        // We want to produce a deterministic hash from all the relevant data in this closure. To do
        // so we 'normalize' the object to remove any meaningless differences, and also to ensure
        // the closure can be appropriately serialized to a JSON string, which can then be sha1
        // hashed.
        //
        // The changes normalization performs are:
        //  1. Cycles are removed.  If a closure is self referenced, we replace it with an object
        //     indicating the reference.
        //  2. The entire structure is ordered (through the use of arrays).  This avoids any
        //     potential concerns around property enumeration order in dictionaries.
        //  3. All data is packed into the final object (even when undefined). That way, if you had
        //     { key: undefined, value: "foo" } and { key: "foo", value: undefined } you don't end
        //     up with the same hash (which would happen if undefined values were ignored, and both
        //     only wrote out the "foo" value).

        // To ensure that cycles are properly represented, keep track of which closures we've seen.
        let seenClosures: Closure[] = [];
        let normalizedClosure = this.convertClosureToNormalizedObject(seenClosures, closure);

        shasum.update(JSON.stringify(normalizedClosure));
        let hash: string = "__" + shasum.digest("hex");
        return hash;
    }

    private convertClosureToNormalizedObject(seenClosures: Closure[], closure: Closure | undefined) {
        if (!closure) {
            return undefined;
        }

        let closureIndex = seenClosures.indexOf(closure);
        if (closureIndex >= 0) {
            // We've already seen this closure.  Represent it specially.  Importantly: do not
            // represent it in the same way that we represent 'no closure' (above).  There is a
            // difference between if we have a cyclic closure versus a non-cyclic one.
            return closureIndex;
        }

        // keep track of this closure so we don't recurse into it again.
        seenClosures.push(closure);

        return [
            closure.code,
            closure.runtime,
            this.convertEnvironmentToNormalizedObject(seenClosures, closure.environment)
        ];
    }

    private convertEnvironmentToNormalizedObject(seenClosures: Closure[], environment: Environment | undefined): any {
        if (!environment) {
            // Encode no environment differently than an empty environment. It may not be necessary
            // to do this.  However, in case there ever is a meaningful distinction between the two,
            // this can help avoid particularly subtle bugs.
            return undefined;
        }

        // Process keys in a deterministic order.
        return Object.keys(environment).sort().map(key => ({
            name: key,
            value: this.convertEnvironmentEntryToNormalizedObject(seenClosures, environment[key]),
        }));
    }

    private convertEnvironmentEntryToNormalizedObject(seenClosures: Closure[], entry: EnvironmentEntry | undefined): any {
        if (!entry) {
            return undefined;
        }

        return [
            entry.json,
            this.convertClosureToNormalizedObject(seenClosures, entry.closure),
            this.convertEnvironmentToNormalizedObject(seenClosures, entry.obj),
            entry.arr ? entry.arr.map(child => this.convertEnvironmentEntryToNormalizedObject(seenClosures, child)) : undefined,
            entry.module
        ];
    }

    private envFromEnvObj(env: Environment): {[key: string]: string} {
        let envObj: {[key: string]: string} = {};
        for (let key of Object.keys(env)) {
            let val = this.envEntryToString(env[key]);
            if (val !== undefined) {
                envObj[key] = val;
            }
        }
        return envObj;
    }

    private envFromEnvArr(arr: EnvironmentEntry[]): (string | undefined)[] {
        let envArr: (string | undefined)[] = [];
        for (let i = 0; i < arr.length; i++) {
            envArr[i] = this.envEntryToString(arr[i]);
        }
        return envArr;
    }

    private envEntryToString(envEntry: EnvironmentEntry): string | undefined {
        if (envEntry.json !== undefined) {
            return JSON.stringify(envEntry.json);
        }
        else if (envEntry.closure !== undefined) {
            let innerHash = this.createFuncForClosure(envEntry.closure);
            return innerHash;
        }
        else if (envEntry.obj !== undefined) {
            return envObjToString(this.envFromEnvObj(envEntry.obj));
        }
        else if (envEntry.arr !== undefined) {
            return envArrToString(this.envFromEnvArr(envEntry.arr));
        }
        else if (envEntry.module !== undefined) {
            return `require("${envEntry.module}")`;
        }
        else {
            return undefined;
        }
    }
}

/**
 * Converts an environment object into a string which can be embedded into a serialized function body.  Note that this
 * is not JSON serialization, as we may have proeprty values which are variable references to other global functions.
 * In other words, there can be free variables in the resulting object literal.
 *
 * @param envObj The environment object to convert to a string.
 */
function envObjToString(envObj: { [key: string]: string; }): string {
    let result = "";
    let first = true;
    for (let key of Object.keys(envObj)) {
        let val = envObj[key];

        // Rewrite references to `this` to the special name `_this`.  This will get rewritten to use `.apply` later.
        if (key === "this") {
            key = "_this";
        }

        if (!first) {
            result += ", ";
        }

        result += key + ": " + val;
        first = false;
    }
    return "{ " + result + " }";
}

function envArrToString(envArr: (string | undefined)[]): string {
    let result = "";
    let first = true;
    for (let i = 0; i < envArr.length; i++) {
        if (!first) {
            result += ", ";
        }
        result += envArr[i];
        first = false;
    }
    return "[ " + result + " ]";
}
