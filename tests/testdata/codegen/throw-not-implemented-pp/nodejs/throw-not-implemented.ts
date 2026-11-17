import * as pulumi from "@pulumi/pulumi";

function notImplemented(message: string): any {
    throw new Error(message);
}

export const result = notImplemented("expression here is not implemented yet");
