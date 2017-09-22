// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { debuggablePromise } from "./debuggable";
import { Log } from "./log";
import * as acorn from "acorn";
import * as estree from "estree";

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
    if (obj === undefined || obj === null ||
            typeof obj === "boolean" || typeof obj === "number" || typeof obj === "string") {
        // Serialize primitives as-is.
        resolve({ json: obj });
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

/**
 * computeFreeVariables computes the set of free variables in a given function string.  Note that this string is
 * expected to be the usual V8-serialized function expression text.
 */
function computeFreeVariables(funcstr: string): string[] {
    Log.debug(`Computing free variables for function: ${funcstr}`);
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
        // Mar references to the built-in 'this' variable as free.
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
            // If the declaration is an identifier, it isn't a free variable, for whatever scope it
            // pertains to (function-wide for var and scope-wide for let/const).  Track it so we can
            // remove any subseqeunt references to that variable, so we know it isn't free.
            if (decl.id.type === "Identifier") {
                let name = (<estree.Identifier>decl.id).name;
                if (node.kind === "var") {
                    this.functionVars.push(name);
                } else {
                    this.scope[this.scope.length-1][name] = true;
                }

                // Make sure to walk the initializer.
                if (decl.init) {
                    cb(decl.init, state);
                }
            } else {
                // If the declaration is something else (say a destructuring pattern), recurse into
                // it so that we can find any other identifiers held within.
                cb(decl, state);
            }
        }
    }
}

