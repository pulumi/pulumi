import * as pulumi from "@pulumi/pulumi";
import * as extbase from "@pulumi/extbase";
import * as myext from "@pulumi/myext";

const prov = new extbase.Provider("prov", {});
const parent = new extbase.Base("parent", {}, {
    provider: prov,
});
// This should inherit prov from parent, because prov is the base provider of myext.
const child = new myext.Greeting("child", {}, {
    parent: parent,
});
