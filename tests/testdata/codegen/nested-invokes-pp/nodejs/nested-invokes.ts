import * as pulumi from "@pulumi/pulumi";
import * as std from "@pulumi/std";

const example = std.upper({
    input: "hello_world",
}).then(invoke => std.replace({
    text: invoke.result,
    search: "_",
    replace: "-",
}));
