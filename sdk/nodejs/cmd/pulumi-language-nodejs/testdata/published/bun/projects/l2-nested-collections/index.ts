import * as pulumi from "@pulumi/pulumi";
import * as nestedcollections from "@pulumi/nestedcollections";

// A resource with deeply nested collection output properties: a list of lists of lists
// of an object type and a map of maps of maps of strings.
const foo = new nestedcollections.Foo("foo", {});
// A resource whose `elementType` property collides with the `ElementType()` method that
// generated Go SDK types must implement.
const elem = new nestedcollections.elementtype.ElementType("elem", {});
export const secondProp = foo.conditionSets[0][0][1].prop;
export const leaf = foo.privateEndpoint.outer.inner.leaf;
export const elementType = elem.elementType.apply(elementType => elementType?.elementType);
