import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const retainOnDelete = new simple.Resource("retainOnDelete", {value: true}, {
    retainOnDelete: true,
});
const notRetainOnDelete = new simple.Resource("notRetainOnDelete", {value: true}, {
    retainOnDelete: false,
});
const defaulted = new simple.Resource("defaulted", {value: true});
