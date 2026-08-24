import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";

const target = new component.ComponentCustomRefOutput("target", {value: "checked"});
const data = component.identityOutput({
    input: "reachable",
}, {
    dependsOn: [target],
});
export const echoed = data.result;
