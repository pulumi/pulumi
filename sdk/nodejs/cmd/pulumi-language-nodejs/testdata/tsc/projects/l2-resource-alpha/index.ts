import * as pulumi from "@pulumi/pulumi";
import * as alpha from "@pulumi/alpha";

const res = new alpha.Resource("res", {value: true});
