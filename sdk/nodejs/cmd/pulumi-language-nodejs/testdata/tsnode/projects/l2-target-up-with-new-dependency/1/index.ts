import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const targetOnly = new simple.Resource("targetOnly", {value: true});
const dep = new simple.Resource("dep", {value: true});
const unrelated = new simple.Resource("unrelated", {value: true}, {
    dependsOn: [dep],
});
