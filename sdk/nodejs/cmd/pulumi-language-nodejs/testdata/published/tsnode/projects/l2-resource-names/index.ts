import * as pulumi from "@pulumi/pulumi";
import * as names from "@pulumi/names";

const res1 = new names.ResMap("res1", {value: true});
const res2 = new names.ResArray("res2", {value: true});
const res3 = new names.ResList("res3", {value: true});
const res4 = new names.ResResource("res4", {value: true});
const res5 = new names.mod.Res("res5", {value: true});
const res6 = new names.mod.nested.Res("res6", {value: true});
