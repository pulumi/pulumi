import * as pulumi from "@pulumi/pulumi";
import * as optional_primitive_ref from "@pulumi/optional-primitive-ref";

const setRes = new optional_primitive_ref.Resource("setRes", {data: {
    boolean: true,
    float: 3.14,
    integer: 42,
    string: "hello",
    numberArray: [
        -1,
        0,
        1,
    ],
    booleanMap: {
        t: true,
        f: false,
    },
}});
const unsetRes = new optional_primitive_ref.Resource("unsetRes", {data: {}});
export const setBoolean = setRes.data.apply(data => data.boolean);
export const setFloat = setRes.data.apply(data => data.float);
export const setInteger = setRes.data.apply(data => data.integer);
export const setString = setRes.data.apply(data => data.string);
export const setNumberArray = setRes.data.apply(data => data.numberArray);
export const setBooleanMap = setRes.data.apply(data => data.booleanMap);
export const unsetBoolean = unsetRes.data.apply(data => data.boolean == null ? "null" : "not null");
export const unsetFloat = unsetRes.data.apply(data => data.float == null ? "null" : "not null");
export const unsetInteger = unsetRes.data.apply(data => data.integer == null ? "null" : "not null");
export const unsetString = unsetRes.data.apply(data => data.string == null ? "null" : "not null");
export const unsetNumberArray = unsetRes.data.apply(data => data.numberArray == null ? "null" : "not null");
export const unsetBooleanMap = unsetRes.data.apply(data => data.booleanMap == null ? "null" : "not null");
