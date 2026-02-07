import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const parent = new simple.Resource("parent", {value: true});
const withParent = new simple.Resource("withParent", {value: false}, {
    parent: parent,
});
const noParent = new simple.Resource("noParent", {value: true});
