import { AssetArchive, String } from "@lumi/lumi/asset"
import { serializeClosure } from "@lumi/lumi/runtime"
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

export class FunctionX {
    private lambda: LambdaFunction;
    private role: Role;

    constructor(name: string, policies: ARN[], func: (event: any, context: Context) => any) {
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
                            // TODO: We need to deserialize these variables, so it can't be as simple as `with`
                            "   with(process.env) {\n" +
                            "       let __f = " + closure.code +
                            "       __f(__event, __context, __callback);\n" +
                            "   }\n" +
                            "}\n"
                        )
                    }),
                    handler: "index.handler",
                    runtime: "nodejs6.10",
                    role: this.role,
                    // TODO: We probably want to put the whole closure environment into a single Lambda
                    // environment variable so that it's easier to extract and deserialize safely.
                    // That will require being able to JSON.stringify.
                    environment: closure.environment,
                });
                break;
            default:
                throw new Error("Language '" + closure.language + "' not yet supported (currently only JavaScript).");
        }
    }
}

