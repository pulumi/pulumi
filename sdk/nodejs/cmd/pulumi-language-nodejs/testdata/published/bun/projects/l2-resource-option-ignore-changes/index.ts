import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const ignoreChanges = new simple.Resource("ignoreChanges", {value: true}, {
    ignoreChanges: ["value"],
});
const notIgnoreChanges = new simple.Resource("notIgnoreChanges", {value: true});
