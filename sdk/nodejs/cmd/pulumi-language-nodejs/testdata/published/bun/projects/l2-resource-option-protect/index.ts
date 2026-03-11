import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const _protected = new simple.Resource("protected", {value: true}, {
    protect: true,
});
const unprotected = new simple.Resource("unprotected", {value: true}, {
    protect: false,
});
const defaulted = new simple.Resource("defaulted", {value: true});
