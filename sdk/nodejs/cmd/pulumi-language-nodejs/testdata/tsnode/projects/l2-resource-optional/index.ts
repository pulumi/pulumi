import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";
import * as simple_optional from "@pulumi/simple-optional";

const resA = new simple.Resource("resA", {value: true});
const resB = new simple_optional.Resource("resB", {value: resA.value});
const resC = new simple_optional.Resource("resC", {
    value: resB.value,
    text: null,
});
