import * as pulumi from "@pulumi/pulumi";
import * as reservednames from "@pulumi/reservednames";

// A resource whose `elementType` property collides with the `ElementType()` method that
// generated Go SDK types must implement.
const elem = new reservednames.ElementType("elem", {elementType: {
    elementType: "nested",
}});
export const elementType = elem.elementType.elementType;
