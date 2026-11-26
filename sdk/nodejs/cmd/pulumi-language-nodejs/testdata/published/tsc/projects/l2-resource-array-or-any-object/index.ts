import * as pulumi from "@pulumi/pulumi";
import * as array_or_any from "@pulumi/array-or-any";

const objectList = new array_or_any.ObjectList("objectList", {values: [
    {
        itemName: "first",
    },
    {
        itemName: "second",
    },
]});
const objectAny = new array_or_any.ObjectList("objectAny", {values: {
    any: true,
}});
