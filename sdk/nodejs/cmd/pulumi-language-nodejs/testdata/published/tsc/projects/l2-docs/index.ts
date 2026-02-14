import * as pulumi from "@pulumi/pulumi";
import * as docs from "@pulumi/docs";

const res = new docs.Resource("res", {"in": docs.funOutput({
    "in": false,
}).apply(invoke => invoke.out)});
