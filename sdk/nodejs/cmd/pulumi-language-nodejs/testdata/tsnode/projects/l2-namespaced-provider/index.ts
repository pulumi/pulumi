import * as pulumi from "@pulumi/pulumi";
import * as namespaced from "@a-namespace/namespaced";

const res = new namespaced.Resource("res", {value: true});
