import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const withV1 = new simple.Resource("withV1", {value: true}, {
    version: "2.0.0",
});
const withV2 = new simple.Resource("withV2", {value: false}, {
    version: "26.0.0",
});
const withDefault = new simple.Resource("withDefault", {value: true});
