import * as pulumi from "@pulumi/pulumi";
import * as output from "@pulumi/output";
import * as simple from "@pulumi/simple";

const replacementTrigger = new simple.Resource("replacementTrigger", {value: true}, {
    replacementTrigger: "test",
});
const unknown = new output.Resource("unknown", {value: 1});
const unknownReplacementTrigger = new simple.Resource("unknownReplacementTrigger", {value: true}, {
    replacementTrigger: "hellohello",
});
const notReplacementTrigger = new simple.Resource("notReplacementTrigger", {value: true});
const secretReplacementTrigger = new simple.Resource("secretReplacementTrigger", {value: true}, {
    replacementTrigger: pulumi.secret([
        1,
        2,
        3,
    ]),
});
