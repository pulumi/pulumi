import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const prov = new simple.Provider("prov", {});
const res = new simple.Resource("res", {value: true}, {
    provider: prov,
});
