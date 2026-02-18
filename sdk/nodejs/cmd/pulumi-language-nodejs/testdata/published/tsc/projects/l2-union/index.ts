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
// List<Union<String, Enum>> pattern
const stringEnumUnionListExample = new union.Example("stringEnumUnionListExample", {stringEnumUnionListProperty: [
    union.AccessRights.Listen,
    union.AccessRights.Send,
    "NotAnEnumValue",
]});
// Safe enum: literal string matching an enum value
const safeEnumExample = new union.Example("safeEnumExample", {typedEnumProperty: union.BlobType.Block});
// Output enum: output from another resource used as enum input
const enumOutputExample = new union.EnumOutput("enumOutputExample", {name: "example"});
const outputEnumExample = new union.Example("outputEnumExample", {typedEnumProperty: enumOutputExample.type});
