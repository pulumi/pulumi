import * as pulumi from "@pulumi/pulumi";
import * as discriminated_union_marked_key from "@pulumi/discriminated-union-marked-key";

const first = new discriminated_union_marked_key.Example("first", {unionIn: {
    discriminantKind: "variant2",
    field2: "known",
}});
const second = new discriminated_union_marked_key.Example("second", {unionIn: first.unionOut});
export const out = second.unionOut;
