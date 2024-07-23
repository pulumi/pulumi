import * as pulumi from "@pulumi/pulumi";
import * as std from "@pulumi/std";

const example = std.replaceOutput({
    text: std.upperOutput({
        input: "hello_world",
    }).apply(invoke => invoke.result),
    search: "_",
    replace: "-",
});
