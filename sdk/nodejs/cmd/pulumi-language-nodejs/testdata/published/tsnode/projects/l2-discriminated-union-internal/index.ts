import * as pulumi from "@pulumi/pulumi";
import * as discriminated_union_internal from "@pulumi/discriminated-union-internal";

const example1 = new discriminated_union_internal.Example("example1", {
    unionOf: {
        __type: "Alpha",
        payload: "p1",
        weight: 1,
    },
    secretUnion: {
        __type: "Beta",
        payload: "s1",
        tint: "blue",
    },
});
const example2 = new discriminated_union_internal.Example("example2", {unionOf: {
    __type: "Beta",
    payload: "p2",
    tint: "red",
}});
const example3 = new discriminated_union_internal.Example("example3", {unionOf: {
    __type: "Gamma",
    payload: "p3",
    active: true,
}});
