import * as pulumi from "@pulumi/pulumi";
import * as union from "@pulumi/union";

const stringOrIntegerExample1 = new union.Example("stringOrIntegerExample1", {stringOrIntegerProperty: 42});
const stringOrIntegerExample2 = new union.Example("stringOrIntegerExample2", {stringOrIntegerProperty: "forty two"});
const mapMapUnionExample = new union.Example("mapMapUnionExample", {mapMapUnionProperty: {
    key1: {
        key1a: "value1a",
    },
}});
export const mapMapUnionOutput = mapMapUnionExample.mapMapUnionProperty;
