import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";
import * as simple from "@pulumi/simple";

const target = new simple.Resource("target", {value: true});
const other = new nestedobject.Container("other", {inputs: ["a"]});
