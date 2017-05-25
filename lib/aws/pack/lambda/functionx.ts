import { AssetArchive, String } from "@lumi/lumi/asset"
import { serializeClosure, jsonStringify } from "@lumi/lumi/runtime"
import { Function as LambdaFunction } from "./function"
import { ARN } from "../types"
import { Role } from "../iam/role";

// Context is the shape of the context object passed to a FunctionX callback.
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

let policy = {
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "sts:AssumeRole",
            "Principal": {
                "Service": "lambda.amazonaws.com"
            },
            "Effect": "Allow",
            "Sid": ""
        }
    ]
}

// FunctionX is a higher-level API for creating and managing AWS Lambda Function resources implemented
// by a Lumi lambda expression and with a set of attached policies.
export class FunctionX {
    private lambda: LambdaFunction;
    private role: Role;

    constructor(name: string, policies: ARN[],
        func: (event: any, context: Context, callback: (error: any, result: any) => void) => any) {

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
        })

        switch (closure.language) {
            case ".js":
                this.lambda = new LambdaFunction(name, {
                    code: new AssetArchive({
                        "index.js": new String(
                            "exports.handler = (__event, __context, __callback) => {\n" +
                            "  let env = JSON.parse(process.env.LUMI_ENV)\n" +
                            // TODO[pulumi/lumi#173]: Once we decide how to deserialize Lumi Resources, we 
                            // likely will need to add additional deserialization behaviour here. 
                            "   with(env) {\n" +
                            "       let __f = " + closure.code +
                            "       __f(__event, __context, __callback);\n" +
                            "   }\n" +
                            "}\n"
                        )
                    }),
                    handler: "index.handler",
                    runtime: "nodejs6.10",
                    role: this.role,
                    environment: {
                        LUMI_ENV: jsonStringify(closure.environment)
                    },
                });
                break;
            default:
                throw new Error("Language '" + closure.language + "' not yet supported (currently only JavaScript).");
        }
    }
}

