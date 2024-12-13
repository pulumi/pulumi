import * as pulumi from "@pulumi/pulumi";
import * as simple_component from "@pulumi/simple-component";

const res = new simple_component.Resource("res", {value: true});
