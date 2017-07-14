// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { AssetArchive, File, String as StringAsset } from "@lumi/lumi/asset";
import {
    Closure, EnvObj, jsonStringify, objectKeys, printf, serializeClosure, sha1hash,
} from "@lumi/lumirt";
import { Role } from "../iam/role";
import { DeadLetterConfig, Function as LambdaFunction } from "../lambda/function";
import { ARN } from "../types";

// Context is the shape of the context object passed to a Function callback.
export interface Context {
    callbackWaitsForEmptyEventLoop: boolean;
    readonly functionName: string;
    readonly functionVersion: string;
    readonly invokedFunctionArn: string;
    readonly memoryLimitInMB: string;
    readonly awsRequestId: string;
    readonly logGroupName: string;
    readonly logStreamName: string;
    readonly identity: any;
    readonly clientContext: any;
    getRemainingTimeInMillis(): string;
}

// Handler is the signature for a serverless function.
export type Handler = (event: any, context: Context, callback: (error: any, result: any) => void) => any;

let policy = {
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "sts:AssumeRole",
            "Principal": {
                "Service": "lambda.amazonaws.com",
            },
            "Effect": "Allow",
            "Sid": "",
        },
    ],
};

interface FuncEnv {
    code: string;
    env: { [key: string]: string; };
}

// FuncsForClosure collects all the function defintions needed to
// support serialization of a given Closure object.  Note that
// a Closure object can reference other Closure objects and
// can also have cycles, so we recursively walk the graph and
// cache serialized nodes along the way to avoid cycles.
class FuncsForClosure {
    public funcs: { [hash: string]: FuncEnv };
    public root: string;

    constructor(closure: Closure) {
        this.funcs = {};
        this.root = this.createFuncForClosure(closure);
    }

    private createFuncForClosure(closure: Closure): string {
        let hash = "__" + sha1hash(closure.code);
        if (this.funcs[hash] === undefined) {
            this.funcs[hash] = {
                code: closure.code,
                env: {}, // initialize as empty - update after recursive call
            };
            this.funcs[hash].env = this.envFromClosureEnvironment(closure.environment);
        }
        return hash;
    }

    private envFromClosureEnvironment(env: EnvObj): {[key: string]: string} {
        let envObj: {[key: string]: string} = {};
        let keys = objectKeys(env);
        for (let i = 0; i < (<any>keys).length; i++) {
            let key = keys[i];
            let envEntry = env[key];
            if (envEntry.json !== undefined) {
                envObj[key] = jsonStringify(envEntry.json);
            } else if (envEntry.closure !== undefined) {
                let innerHash = this.createFuncForClosure(envEntry.closure);
                envObj[key] = innerHash;
            } else if (envEntry.obj !== undefined) {
                envObj[key] = envObjToString(this.envFromClosureEnvironment(envEntry.obj));
            } else {
                // TODO[pulumi/lumi#239]: For now we will skip serialziing when the captured JSON object is
                // null/undefined. This is not technically correct, as it will cause references to these to
                // fail instead of return undefined.
            }
        }
        return envObj;
    }
}

// Converts an environment object into a string which can be embedded into a serialized
// function body.  Note that this is not JSON serialization, as we may have proeprty
// values which are variable references to other global functions.  In other words,
// there can be free variables in the resulting object literal.
function envObjToString(envObj: { [key: string]: string; }): string {
    let ret = "{";
    let isStart = true;
    let keys = objectKeys(envObj);
    for (let i = 0; i < (<any>keys).length; i++) {
        let key = keys[i];
        let val = envObj[key];
        // Lumi generates the special name `.this` for references to `this`.
        // We will rewrite to the name `_this` and then pass that as the
        // receiver to `.apply` later on.
        if (key === ".this") {
            key = "_this";
        }
        if (isStart) {
            ret += " ";
        } else {
            ret += ", ";
        }
        isStart = false;
        ret += key + ": " + val;
    }
    ret += " }";
    return ret;
}

function createJavaScriptLambda(
    functionName: string,
    role: Role,
    closure: Closure,
    opts: FunctionOptions): LambdaFunction {

    let funcsForClosure = new FuncsForClosure(closure);
    let funcs = funcsForClosure.funcs;
    let str = "exports.handler = " + funcsForClosure.root + ";\n\n";
    let fkeys = objectKeys(funcs);
    for (let i = 0; i < (<any>fkeys).length; i++) {
        let name = fkeys[i];
        str +=
            "function " + name + "() {\n" +
            "  var _this;\n" +
            "  with(" + envObjToString(funcs[name].env) + ") {\n" +
            "    return (function() {\n\n" +
            funcs[name].code + "\n" +
            "    }).apply(_this).apply(undefined, arguments);\n" +
            "  }\n" +
            "}\n" +
            "\n";
    }

    // Inject some TypeScript runtime helpers that the transpiled code may have dependencies on.
    // These are necessary for targeting Node.js runtime environments that do not yet support
    // new ECMAScript features like `async`/`await`.
    //
    // The implementations are sourced from: https://github.com/Microsoft/tslib/blob/master/tslib.es6.js
    /*tslint:disable: max-line-length */
    str += `
function __awaiter(thisArg, _arguments, P, generator) {
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator.throw(value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
}

function __generator(thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g;
    return g = { next: verb(0), "throw": verb(1), "return": verb(2) }, typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (_) try {
            if (f = 1, y && (t = y[op[0] & 2 ? "return" : op[0] ? "throw" : "next"]) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [0, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
}
`;

    let timeout = 180;
    if (opts.timeout !== undefined) {
        timeout = opts.timeout;
    }

    let lambda = new LambdaFunction(functionName, {
        code: new AssetArchive({
            "node_modules": new File("node_modules"),
            "index.js": new StringAsset(str),
        }),
        handler: "index.handler",
        runtime: "nodejs6.10",
        role: role,
        timeout: timeout,
        memorySize: opts.memorySize,
        deadLetterConfig: opts.deadLetterConfig,
    });

    return lambda;
}

export interface FunctionOptions {
    policies: ARN[];
    timeout?: number;
    memorySize?: number;
    deadLetterConfig?: DeadLetterConfig;
}

// Function is a higher-level API for creating and managing AWS Lambda Function resources implemented
// by a Lumi lambda expression and with a set of attached policies.
export class Function {
    public lambda: LambdaFunction;
    public role: Role;

    constructor(name: string, options: FunctionOptions, func: Handler) {
        if (name === undefined) {
            throw new Error("Missing required resource name");
        }
        if (func === undefined) {
            throw new Error("Missing required function callback");
        }
        let closure = serializeClosure(func);
        if (closure === undefined) {
            throw new Error("Failed to serialize function.");
        }

        this.role = new Role(name + "-role", {
            assumeRolePolicyDocument: policy,
            managedPolicyARNs: options.policies,
        });

        switch (closure.language) {
            case ".js":
                this.lambda = createJavaScriptLambda(name, this.role, closure, options);
                break;
            default:
                throw new Error("Language '" + closure.language + "' not yet supported (currently only JavaScript).");
        }
    }
}

