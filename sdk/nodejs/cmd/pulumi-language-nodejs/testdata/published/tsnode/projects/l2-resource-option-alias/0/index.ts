import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

// Make a simple resource to use as a parent
const parent = new simple.Resource("parent", {value: true});
const aliasURN = new simple.Resource("aliasURN", {value: true});
const aliasName = new simple.Resource("aliasName", {value: true});
const aliasNoParent = new simple.Resource("aliasNoParent", {value: true});
const aliasParent = new simple.Resource("aliasParent", {value: true}, {parent: aliasURN});
