import * as pulumi from "@pulumi/pulumi";
import * as child_process from "child_process";
import * as simple from "@pulumi/simple";

function notImplemented(message: string): any {
    throw new Error(message);
}

const panicHook = new pulumi.ResourceHook("panicHook", (args) => {
    child_process.execFileSync(notImplemented("hook panic"), []);
});
const res = new simple.Resource("res", {value: true}, {
    hooks: {
        afterCreate: [panicHook],
    },
});
