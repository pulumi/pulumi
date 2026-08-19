// Copyright 2025, Pulumi Corporation.  All rights reserved.

import { boundedFor, cond } from "@pulumi/pulumi";
import { Resource } from "./resource";

let a = new Resource("res", { arg: "hello" });

let check = a.state.apply(v => v == "hello world");

let b = cond(check, () => {
    let c = new Resource("res2", { arg: "au revoir" });
    return c.state;
}, () => {
    let c = new Resource("res3", { arg: "ciao" });
    return c.state;
});

let cs = b.apply(v => v.split("").slice(0, 5));
let d = boundedFor(cs, 10, (v, i) => {
    var res = new Resource(`res4-${i}`, { arg: v });
    return res.state;
});

export let o = b;
export let os = d;
