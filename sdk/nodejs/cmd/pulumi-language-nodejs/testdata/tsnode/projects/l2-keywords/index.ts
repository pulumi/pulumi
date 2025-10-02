import * as pulumi from "@pulumi/pulumi";
import * as keywords from "@pulumi/keywords";

const firstResource = new keywords.SomeResource("firstResource", {
    builtins: "builtins",
    property: "property",
});
const secondResource = new keywords.SomeResource("secondResource", {
    builtins: firstResource.builtins,
    property: firstResource.property,
});
