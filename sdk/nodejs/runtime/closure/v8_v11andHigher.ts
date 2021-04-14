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

// tslint:disable:max-line-length

import * as inspector from "inspector";
import * as util from "util";
import * as vm from "vm";
import * as v8Hooks from "./v8Hooks";

/** @internal */
export async function getFunctionLocationAsync(func: Function) {
    // First, find the runtime's internal id for this function.
    const functionId = await getRuntimeIdForFunctionAsync(func);

    // Now, query for the internal properties the runtime sets up for it.
    const { internalProperties } = await runtimeGetPropertiesAsync(functionId, /*ownProperties:*/ false);

    // There should normally be an internal property called [[FunctionLocation]]:
    // https://chromium.googlesource.com/v8/v8.git/+/3f99afc93c9ba1ba5df19f123b93cc3079893c9b/src/inspector/v8-debugger.cc#793
    const functionLocation = internalProperties.find(p => p.name === "[[FunctionLocation]]");
    if (!functionLocation || !functionLocation.value || !functionLocation.value.value) {
        return { file: "", line: 0, column: 0 };
    }

    const value = functionLocation.value.value;

    // Map from the scriptId the value has to a file-url.
    const file = v8Hooks.getScriptUrl(value.scriptId) || "";
    const line = value.lineNumber || 0;
    const column = value.columnNumber || 0;

    return { file, line, column };
}

/** @internal */
export async function lookupCapturedVariableValueAsync(
    func: Function, freeVariable: string, throwOnFailure: boolean): Promise<any> {

    // First, find the runtime's internal id for this function.
    const functionId = await getRuntimeIdForFunctionAsync(func);

    // Now, query for the internal properties the runtime sets up for it.
    const { internalProperties } = await runtimeGetPropertiesAsync(functionId, /*ownProperties:*/ false);

    // There should normally be an internal property called [[Scopes]]:
    // https://chromium.googlesource.com/v8/v8.git/+/3f99afc93c9ba1ba5df19f123b93cc3079893c9b/src/inspector/v8-debugger.cc#820
    const scopes = internalProperties.find(p => p.name === "[[Scopes]]");
    if (!scopes) {
        throw new Error("Could not find [[Scopes]] property");
    }

    if (!scopes.value) {
        throw new Error("[[Scopes]] property did not have [value]");
    }

    if (!scopes.value.objectId) {
        throw new Error("[[Scopes]].value have objectId");
    }

    // This is sneaky, but we can actually map back from the [[Scopes]] object to a real in-memory
    // v8 array-like value.  Note: this isn't actually a real array.  For example, it cannot be
    // iterated.  Nor can any actual methods be called on it. However, we can directly index into
    // it, and we can.  Similarly, the 'object' type it optionally points at is not a true JS
    // object.  So we can't call things like .hasOwnProperty on it.  However, the values pointed to
    // by 'object' are the real in-memory JS objects we are looking for.  So we can find and return
    // those successfully to our caller.
    const scopesArray: { object?: Record<string, any> }[] = await getValueForObjectId(scopes.value.objectId);

    // scopesArray is ordered from innermost to outermost.
    for (let i = 0, n = scopesArray.length; i < n; i++) {
        const scope = scopesArray[i];
        if (scope.object) {
            if (freeVariable in scope.object) {
                const val = scope.object[freeVariable];
                return val;
            }
        }
    }

    if (throwOnFailure) {
        throw new Error("Unexpected missing variable in closure environment: " + freeVariable);
    }

    return undefined;
}

// We want to call util.promisify on inspector.Session.post. However, due to all the overloads of
// that method, promisify gets confused.  To prevent this, we cast our session object down to an
// interface containing only the single overload we care about.
type PostSession<TMethod, TParams, TReturn> = {
    post(method: TMethod, params?: TParams, callback?: (err: Error | null, params: TReturn) => void): void;
};

type EvaluationSession = PostSession<"Runtime.evaluate", inspector.Runtime.EvaluateParameterType, inspector.Runtime.EvaluateReturnType>;
type GetPropertiesSession = PostSession<"Runtime.getProperties", inspector.Runtime.GetPropertiesParameterType, inspector.Runtime.GetPropertiesReturnType>;
type CallFunctionSession = PostSession<"Runtime.callFunctionOn", inspector.Runtime.CallFunctionOnParameterType, inspector.Runtime.CallFunctionOnReturnType>;
type ContextSession = {
    post(method: "Runtime.disable" | "Runtime.enable", callback?: (err: Error | null) => void): void;
    once(event: "Runtime.executionContextCreated", listener: (message: inspector.InspectorNotification<inspector.Runtime.ExecutionContextCreatedEventDataType>) => void): void;
};

type InflightContext = {
    contextId: number;
    functions: Record<string, any>;
    currentFunctionId: number;
    calls: Record<string, any>;
    currentCallId: number;
};
// Isolated singleton context accessible from the inspector.
// Used instead of `global` object to support executions with multiple V8 vm contexts as, e.g., done by Jest.
const inflightContext = createContext();
async function createContext(): Promise<InflightContext> {
    const context: InflightContext = {
        contextId: 0,
        functions: {},
        currentFunctionId: 0,
        calls: {},
        currentCallId: 0,
    };
    const session = <ContextSession>await v8Hooks.getSessionAsync();
    const post = util.promisify(session.post);

    // Create own context with known context id and functionsContext as `global`
    await post.call(session, "Runtime.enable");
    const contextIdAsync = new Promise<number>(resolve => {
        session.once("Runtime.executionContextCreated", event => {
            resolve(event.params.context.id);
        });
    });
    vm.createContext(context);
    context.contextId = await contextIdAsync;
    await post.call(session, "Runtime.disable");

    return context;
}

async function getRuntimeIdForFunctionAsync(func: Function): Promise<inspector.Runtime.RemoteObjectId> {
    // In order to get information about an object, we need to put it in a well known location so
    // that we can call Runtime.evaluate and find it.  To do this, we use a special map on the
    // 'global' object of a vm context only used for this purpose, and map from a unique-id to that
    // object.  We then call Runtime.evaluate with an expression that then points to that unique-id
    // in that global object.  The runtime will then find the object and give us back an internal id
    // for it.  We can then query for information about the object through that internal id.
    //
    // Note: the reason for the mapping object and the unique-id we create is so that we don't run
    // into any issues when being called asynchronously.  We don't want to place the object in a
    // location that might be overwritten by another call while we're asynchronously waiting for our
    // original call to complete.

    const session = <EvaluationSession>await v8Hooks.getSessionAsync();
    const post = util.promisify(session.post);

    // Place the function in a unique location
    const context = await inflightContext;
    const currentFunctionName = "id" + context.currentFunctionId++;
    context.functions[currentFunctionName] = func;
    const contextId = context.contextId;
    const expression = `functions.${currentFunctionName}`;

    try {
        const retType = await post.call(session, "Runtime.evaluate", { contextId, expression });

        if (retType.exceptionDetails) {
            throw new Error(`Error calling "Runtime.evaluate(${expression})" on context ${contextId}: ` + retType.exceptionDetails.text);
        }

        const remoteObject = retType.result;
        if (remoteObject.type !== "function") {
            throw new Error("Remote object was not 'function': " + JSON.stringify(remoteObject));
        }

        if (!remoteObject.objectId) {
            throw new Error("Remote function does not have 'objectId': " + JSON.stringify(remoteObject));
        }

        return remoteObject.objectId;
    }
    finally {
        delete context.functions[currentFunctionName];
    }
}

async function runtimeGetPropertiesAsync(
        objectId: inspector.Runtime.RemoteObjectId,
        ownProperties: boolean | undefined) {
    const session = <GetPropertiesSession>await v8Hooks.getSessionAsync();
    const post = util.promisify(session.post);

    // This cast will become unnecessary when we move to TS 3.1.6 or above.  In that version they
    // support typesafe '.call' calls.
    const retType = <inspector.Runtime.GetPropertiesReturnType>await post.call(
        session, "Runtime.getProperties", { objectId, ownProperties });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling "Runtime.getProperties(${objectId}, ${ownProperties})": `
            + retType.exceptionDetails.text);
    }

    return { internalProperties: retType.internalProperties || [], properties: retType.result };
}

async function getValueForObjectId(objectId: inspector.Runtime.RemoteObjectId): Promise<any> {
    // In order to get the raw JS value for the *remote wrapper* of the [[Scopes]] array, we use
    // Runtime.callFunctionOn on it passing in a fresh function-declaration.  The Node runtime will
    // then compile that function, invoking it with the 'real' underlying scopes-array value in
    // memory as the bound 'this' value.  Inside that function declaration, we can then access
    // 'this' and assign it to a unique-id in a well known mapping table we have set up.  As above,
    // the unique-id is to prevent any issues with multiple in-flight asynchronous calls.

    const session = <CallFunctionSession>await v8Hooks.getSessionAsync();
    const post = util.promisify(session.post);
    const context = await inflightContext;

    // Get an id for an unused location in the global table.
    const tableId = "id" + context.currentCallId++;

    // Now, ask the runtime to call a fictitious method on the scopes-array object.  When it
    // does, it will get the actual underlying value for the scopes array and bind it to the
    // 'this' value inside the function.  Inside the function we then just grab 'this' and
    // stash it in our global table.  After this completes, we'll then have access to it.

    // This cast will become unnecessary when we move to TS 3.1.6 or above.  In that version they
    // support typesafe '.call' calls.
    const retType = <inspector.Runtime.CallFunctionOnReturnType>await post.call(
        session, "Runtime.callFunctionOn", {
            objectId,
            functionDeclaration: `function () {
                calls["${tableId}"] = this;
            }`,
        });

    if (retType.exceptionDetails) {
        throw new Error(`Error calling "Runtime.callFunction(${objectId})": `
            + retType.exceptionDetails.text);
    }

    if (!context.calls.hasOwnProperty(tableId)) {
        throw new Error(`Value was not stored into table after calling "Runtime.callFunctionOn(${objectId})"`);
    }

    // Extract value and clear our table entry.
    const val = context.calls[tableId];
    delete context.calls[tableId];

    return val;
}
