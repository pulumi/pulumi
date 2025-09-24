import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const res = new simple.Resource("res", {value: false});
