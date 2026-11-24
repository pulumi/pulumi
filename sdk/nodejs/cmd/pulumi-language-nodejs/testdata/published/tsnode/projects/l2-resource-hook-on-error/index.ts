import * as pulumi from "@pulumi/pulumi";
import * as child_process from "child_process";
import * as flaky from "@pulumi/flaky";

const config = new pulumi.Config();
const hookTestFile = config.require("hookTestFile");
const retryHook = new pulumi.ErrorHook("retryHook", (args) => {
    try {
        child_process.execFileSync("touch", [hookTestFile]);
        return true;
    } catch (error) {
        return false;
    }
});
const res = new flaky.FlakyCreate("res", {}, {
    hooks: {
        onError: [retryHook],
    },
});
