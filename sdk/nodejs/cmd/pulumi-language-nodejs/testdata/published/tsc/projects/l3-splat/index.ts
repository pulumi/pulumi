import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";
import * as optional_primitive_ref from "@pulumi/optional-primitive-ref";

const source = new nestedobject.Container("source", {inputs: [
    "a",
    "b",
]});
const sink = new nestedobject.Container("sink", {inputs: source.details.apply(details => details.map(__item => __item.value))});
const setOptional = new optional_primitive_ref.Resource("setOptional", {
    data: {},
    optionalData: {
        string: "set",
    },
});
const unsetOptional = new optional_primitive_ref.Resource("unsetOptional", {data: {}});
export const setOptionalSplat = setOptional.optionalData.apply(optionalData => [optionalData].filter(__value => __value != null).map(__value => (__value)).map(__item => __item?.string));
export const unsetOptionalSplat = unsetOptional.optionalData.apply(optionalData => [optionalData].filter(__value => __value != null).map(__value => (__value)).map(__item => __item?.string));
