import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const resY = new simple.Resource("resY", {value: true});
const resN = new simple.Resource("resN", {value: false});
