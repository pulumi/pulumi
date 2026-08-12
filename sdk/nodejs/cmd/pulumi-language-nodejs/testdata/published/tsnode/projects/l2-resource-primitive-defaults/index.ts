import * as pulumi from "@pulumi/pulumi";
import * as primitive_defaults from "@pulumi/primitive-defaults";

const resExplicit = new primitive_defaults.Resource("resExplicit", {
    boolean: true,
    float: 3.14,
    integer: 42,
    string: "hello",
});
const resDefaulted = new primitive_defaults.Resource("resDefaulted", {});
