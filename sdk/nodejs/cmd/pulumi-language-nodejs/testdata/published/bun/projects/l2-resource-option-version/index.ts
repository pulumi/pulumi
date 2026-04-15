import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const withV2 = new simple.Resource("withV2", {value: true}, {
    version: "2.0.0",
});
const withV26 = new simple.Resource("withV26", {value: false});
const withDefault = new simple.Resource("withDefault", {value: true});
