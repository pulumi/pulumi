import * as pulumi from "@pulumi/pulumi";
import * as child_process from "child_process";
import * as simple from "@pulumi/simple";

const failingHook = new pulumi.ResourceHook("failingHook", (args) => {
    child_process.execFileSync("false", []);
});
const res = new simple.Resource("res", {value: true}, {
    hooks: {
        afterCreate: [failingHook],
    },
});
