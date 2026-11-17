import * as pulumi from "@pulumi/pulumi";
import * as selfref from "@pulumi/selfref";

const root = new selfref.Node("root", {});
const child = new selfref.Node("child", {
    parent: root,
    parents: [root],
    namedParents: {
        root: root,
    },
    parentOrName: root,
});
