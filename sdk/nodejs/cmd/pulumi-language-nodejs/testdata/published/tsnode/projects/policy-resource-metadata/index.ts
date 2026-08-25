import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const prov = new simple.Provider("prov", {});
const identity = new simple.Resource("identity", {value: true});
const _protected = new simple.Resource("protected", {value: true}, {
    protect: true,
});
const ignoreChanges = new simple.Resource("ignoreChanges", {value: true}, {
    ignoreChanges: ["value"],
});
const deleteBeforeReplace = new simple.Resource("deleteBeforeReplace", {value: true}, {
    deleteBeforeReplace: true,
});
const secretOutput = new simple.Resource("secretOutput", {value: true}, {
    additionalSecretOutputs: ["value"],
});
const customTimeouts = new simple.Resource("customTimeouts", {value: true}, {
    customTimeouts: {
        create: "5m",
    },
});
const explicitProvider = new simple.Resource("explicitProvider", {value: true}, {
    provider: prov,
});
const parent = new simple.Resource("parent", {value: true});
const child = new simple.Resource("child", {value: true}, {
    parent: parent,
});
const dependency = new simple.Resource("dependency", {value: true});
const dependsOn = new simple.Resource("dependsOn", {value: true}, {
    dependsOn: [dependency],
});
const propertyDependency = new simple.Resource("propertyDependency", {value: dependency.value});
