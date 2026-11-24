import * as pulumi from "@pulumi/pulumi";
import * as optionalprimitive from "@pulumi/optionalprimitive";
import * as primitive from "@pulumi/primitive";

const unsetA = new optionalprimitive.Resource("unsetA", {});
const unsetB = new optionalprimitive.Resource("unsetB", {
    boolean: unsetA.boolean,
    float: unsetA.float,
    integer: unsetA.integer,
    string: unsetA.string,
    numberArray: unsetA.numberArray,
    booleanMap: unsetA.booleanMap,
});
export const unsetBoolean = unsetB.boolean.apply(boolean => boolean == null ? "null" : "not null");
export const unsetFloat = unsetB.float.apply(float => float == null ? "null" : "not null");
export const unsetInteger = unsetB.integer.apply(integer => integer == null ? "null" : "not null");
export const unsetString = unsetB.string.apply(string => string == null ? "null" : "not null");
export const unsetNumberArray = unsetB.numberArray.apply(numberArray => numberArray == null ? "null" : "not null");
export const unsetBooleanMap = unsetB.booleanMap.apply(booleanMap => booleanMap == null ? "null" : "not null");
const setA = new optionalprimitive.Resource("setA", {
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
});
const setB = new optionalprimitive.Resource("setB", {
    boolean: setA.boolean,
    float: setA.float,
    integer: setA.integer,
    string: setA.string,
    numberArray: setA.numberArray,
    booleanMap: setA.booleanMap,
});
const sourcePrimitive = new primitive.Resource("sourcePrimitive", {
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
});
const fromPrimitive = new optionalprimitive.Resource("fromPrimitive", {
    boolean: sourcePrimitive.boolean,
    float: sourcePrimitive.float,
    integer: sourcePrimitive.integer,
    string: sourcePrimitive.string,
    numberArray: sourcePrimitive.numberArray,
    booleanMap: sourcePrimitive.booleanMap,
});
export const setBoolean = setB.boolean;
export const setFloat = setB.float;
export const setInteger = setB.integer;
export const setString = setB.string;
export const setNumberArray = setB.numberArray;
export const setBooleanMap = setB.booleanMap;
