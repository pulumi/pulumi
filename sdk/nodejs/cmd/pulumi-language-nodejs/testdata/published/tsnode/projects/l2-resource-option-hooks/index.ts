import * as pulumi from "@pulumi/pulumi";
import * as child_process from "child_process";
import * as simple from "@pulumi/simple";

const config = new pulumi.Config();
const hookTestFile = config.require("hookTestFile");
const hookPreviewFile = config.require("hookPreviewFile");
const createHook = new pulumi.ResourceHook("createHook", (args) => {
    child_process.execFileSync("touch", [hookTestFile]);
});
const previewHook = new pulumi.ResourceHook("previewHook", (args) => {
    child_process.execFileSync("touch", [`${hookPreviewFile}_${args.name}`]);
}, {onDryRun: true});
const res = new simple.Resource("res", {value: true}, {
    hooks: {
        beforeCreate: [createHook, previewHook],
    },
});
