import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const resA = new simple.Resource("resA", {value: true});
const resB = new simple.Resource("resB", {value: false});
const resC = new simple.Resource("resC", {value: false});
