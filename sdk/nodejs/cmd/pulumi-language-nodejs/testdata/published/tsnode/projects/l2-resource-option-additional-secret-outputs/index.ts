import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const withSecret = new simple.Resource("withSecret", {value: true}, {
    additionalSecretOutputs: ["value"],
});
const withoutSecret = new simple.Resource("withoutSecret", {value: true});
