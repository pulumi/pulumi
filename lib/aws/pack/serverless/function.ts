// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { AssetArchive, String, File } from "@lumi/lumi/asset";
import {
    serializeClosure, jsonStringify, objectKeys, Closure, printf
} from "@lumi/lumirt";
import { Role } from "../iam/role";
import { Function as LambdaFunction } from "../lambda/function";
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
    env: string;
}

// addToFuncEnvs adds the closure with the given name into a collection of function environments.  As it walks the 
// closures environment, it finds additional closures that this closure is dependent on, and recursively adds those
// to the function environment as well.  The resulting environments for each closure are json stringified representations
// ready to be marshalled to a target execution environment.
function addToFuncEnvs(funcEnvs: { [key: string]: FuncEnv}, name: string, closure: Closure): { [key: string]: FuncEnv} {
    let obj: any = {}
    let keys = objectKeys(closure.environment);
    for (let i = 0; i < (<any>keys).length; i++) {
        let key = keys[i];
        let envEntry = closure.environment[key];
        if (envEntry.json !== undefined) {
            obj[key] = envEntry.json;
        } else if (envEntry.closure !== undefined) {
            // TODO[pulumi/lumi#238]: We need to detect cycles here.
            addToFuncEnvs(funcEnvs, key, envEntry.closure)
        } else {
            // TODO[pulumi/lumi#239]: For now we will skip serialziing when the captured JSON object is null/undefined.
            //     This is not technically correct, as it will cause references to these to fail instead
            //     of return undefined.
        }
    }
    funcEnvs[name] = {
        code: closure.code,
        env: jsonStringify(obj),
    };
    return funcEnvs;
}

function createJavaScriptLambda(functionName: string, role: Role, closure: Closure): LambdaFunction {
    let funcs = addToFuncEnvs({}, "__handler", closure);
    
    let str = "exports.handler = __handler;\n\n";
    let fkeys = objectKeys(funcs);
    let envObj: any = {}
    for (let i = 0; i < (<any>fkeys).length; i++) { 
        let name = fkeys[i];
        str += 
            "function " + name + "() {\n" +
            "  let __env = JSON.parse(process.env.LUMI_ENV_" + name + ");\n" +
            "  with(__env) {\n" +
            "    let __f = " + funcs[name].code +
            "    return __f.apply(null, arguments);\n" +
            "  }\n" + 
            "}\n" + 
            "\n";
        envObj["LUMI_ENV_" + name] = funcs[name].env;
    }

    let lambda = new LambdaFunction(functionName, {
        code: new AssetArchive({
            "node_modules": new File("node_modules"),
            "index.js": new String(str)
        }),
        handler: "index.handler",
        runtime: "nodejs6.10",
        role: role,
        timeout: 180,
        environment: envObj,
    });

    return lambda;
}

// Function is a higher-level API for creating and managing AWS Lambda Function resources implemented
// by a Lumi lambda expression and with a set of attached policies.
export class Function {
    public lambda: LambdaFunction;
    public role: Role;

    constructor(name: string, policies: ARN[], func: Handler) {
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
            managedPolicyARNs: policies,
        });

        switch (closure.language) {
            case ".js":
                this.lambda = createJavaScriptLambda(name, this.role, closure);
                break;
            default:
                throw new Error("Language '" + closure.language + "' not yet supported (currently only JavaScript).");
        }
    }
}

