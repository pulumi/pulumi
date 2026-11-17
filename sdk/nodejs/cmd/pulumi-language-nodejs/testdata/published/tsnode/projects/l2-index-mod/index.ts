import * as pulumi from "@pulumi/pulumi";
import * as index_mod from "@pulumi/index-mod";

const res1 = new index_mod.indexmine.Resource("res1", {text: index_mod.indexmine.concatWorldOutput({
    value: "hello",
}).result});
export const out1 = res1.call(({
    input: "x",
})).output;
const res2 = new index_mod.indexmine.nested.Resource("res2", {text: index_mod.indexmine.nested.concatWorldOutput({
    value: "goodbye",
}).result});
export const out2 = res2.call(({
    input: "xx",
})).output;
