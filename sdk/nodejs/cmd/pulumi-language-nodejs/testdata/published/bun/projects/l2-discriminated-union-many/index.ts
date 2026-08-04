import * as pulumi from "@pulumi/pulumi";
import * as discriminated_union_many from "@pulumi/discriminated-union-many";

const example1 = new discriminated_union_many.Example("example1", {unionOf: {
    discriminantKind: "variant1",
    payload: "p1",
    extra: "e1",
}});
const example2 = new discriminated_union_many.Example("example2", {unionOf: {
    discriminantKind: "variant2",
    payload: "p2",
    extra: "e2",
}});
const example3 = new discriminated_union_many.Example("example3", {unionOf: {
    discriminantKind: "variant3",
    payload: "p3",
    count: 3,
}});
const example4 = new discriminated_union_many.Example("example4", {unionOf: {
    discriminantKind: "variant4",
    payload: "p4",
    enabled: true,
}});
const example5 = new discriminated_union_many.Example("example5", {unionOf: {
    discriminantKind: "variant5",
    payload: "p5",
    label: "l5",
}});
const example6 = new discriminated_union_many.Example("example6", {unionOf: {
    discriminantKind: "variant6",
    payload: "p6",
    code: 6,
}});
const example7 = new discriminated_union_many.Example("example7", {unionOf: {
    discriminantKind: "variant7",
    payload: "p7",
    message: "m7",
}});
const example8 = new discriminated_union_many.Example("example8", {unionOf: {
    discriminantKind: "variant8",
    payload: "p8",
    size: 8,
}});
const example9 = new discriminated_union_many.Example("example9", {unionOf: {
    discriminantKind: "variant9",
    payload: "p9",
    flag: false,
}});
const example10 = new discriminated_union_many.Example("example10", {unionOf: {
    discriminantKind: "variant10",
    payload: "p10",
    note: "n10",
}});
