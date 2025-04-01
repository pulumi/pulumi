import * as pulumi from "@pulumi/pulumi";
import * as namespaced from "@a-namespace/namespaced";
import * as simple from "@pulumi/simple";

const simpleRes = new simple.Resource("simpleRes", {value: true});
const res = new namespaced.Resource("res", {value: true});
