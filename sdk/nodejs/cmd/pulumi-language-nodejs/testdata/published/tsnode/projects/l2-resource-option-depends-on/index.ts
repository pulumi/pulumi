import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const noDependsOn = new simple.Resource("noDependsOn", {value: true});
const withDependsOn = new simple.Resource("withDependsOn", {value: false}, {
    dependsOn: [noDependsOn],
});
