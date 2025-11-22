import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const replacementTrigger = new simple.Resource("replacementTrigger", {value: true}, {
    replacementTrigger: "test",
});
const notReplacementTrigger = new simple.Resource("notReplacementTrigger", {value: true});
