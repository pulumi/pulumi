import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const target = new simple.Resource("target", {value: true});
const replaceWith = new simple.Resource("replaceWith", {value: true}, {
    replaceWith: target,
});
const notReplaceWith = new simple.Resource("notReplaceWith", {value: true});
