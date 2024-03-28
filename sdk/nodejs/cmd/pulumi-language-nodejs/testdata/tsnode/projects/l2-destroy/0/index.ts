import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const aresource = new simple.Resource("aresource", {value: true});
const other = new simple.Resource("other", {value: true});
