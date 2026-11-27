import * as pulumi from "@pulumi/pulumi";
import * as array_or_any from "@pulumi/array-or-any";

const stringList = new array_or_any.StringList("stringList", {values: [
    "a",
    "b",
    "c",
]});
const stringAny = new array_or_any.StringList("stringAny", {values: {
    any: true,
}});
