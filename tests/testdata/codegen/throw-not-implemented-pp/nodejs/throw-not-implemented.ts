import * as pulumi from "@pulumi/pulumi";

function notImplemented(message: string) {
    throw new Error(message);
}

export const result = notImplemented("expression here is not implemented yet");
