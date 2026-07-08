import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const config = new pulumi.Config();
const createBool = config.requireBoolean("createBool");
let boolResource: nestedobject.Target | undefined;
if (createBool) {
    boolResource = new nestedobject.Target("boolResource", {name: "bool-resource"});
}
const boolTarget = new nestedobject.Target("boolTarget", {name: boolResource!.name.apply(name => `${name}+`)});
