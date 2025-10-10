import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const target = new simple.Resource("target", {value: true});
const deletedWith = new simple.Resource("deletedWith", {value: true}, {
    deletedWith: target,
});
const notDeletedWith = new simple.Resource("notDeletedWith", {value: true});
