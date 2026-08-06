import * as pulumi from "@pulumi/pulumi";
import * as discriminated_union_internal from "@pulumi/discriminated-union-internal";

const example1 = new discriminated_union_internal.Example("example1", {
    unionOf: {
        type__: "Alpha",
        payload: "p1",
        weight: 1,
    },
    secretUnion: {
        type__: "Beta",
        payload: "s1",
        tint: "blue",
    },
});
const example2 = new discriminated_union_internal.Example("example2", {unionOf: {
    type__: "Beta",
    payload: "p2",
    tint: "red",
}});
const example3 = new discriminated_union_internal.Example("example3", {unionOf: {
    type__: "Gamma",
    payload: "p3",
    active: true,
}});
