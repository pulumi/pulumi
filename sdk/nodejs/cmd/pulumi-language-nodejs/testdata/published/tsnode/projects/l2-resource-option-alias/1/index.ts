import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const parent = new simple.Resource("parent", {value: true});
const aliasURN = new simple.Resource("aliasURN", {value: true}, {
    aliases: ["urn:pulumi:test::l2-resource-option-alias::simple:index:Resource::aliasURN"],
    parent: parent,
});
const aliasNewName = new simple.Resource("aliasNewName", {value: true}, {
    aliases: [{
        name: "aliasName",
    }],
});
