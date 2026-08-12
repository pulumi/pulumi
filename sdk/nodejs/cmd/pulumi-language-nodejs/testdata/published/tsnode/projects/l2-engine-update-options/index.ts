import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const target = new simple.Resource("target", {value: true});
const other = new simple.Resource("other", {value: true});
