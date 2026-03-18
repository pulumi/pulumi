import * as pulumi from "@pulumi/pulumi";
import { First } from "./first";
import { Second } from "./second";

// first & second are simple mutually dependent components
const [secondUntainted, resolveSecondUntainted] = pulumi.deferredOutput<boolean>();
const first = new First("first", {input: secondUntainted});
const second = new Second("second", {input: first.untainted});
resolveSecondUntainted(second.untainted);
// another & many are also mutually dependent components, but many tests that the mutual dependency works through
// `range`.
const [loopingOverMany, resolveLoopingOverMany] = pulumi.deferredOutput<Array<string>>();
const another = new First("another", {input: loopingOverMany.apply(loopingOverMany => loopingOverMany.join("")).apply(join => join == "xyz")});
const many: Second[] = [];
for (const range = {value: 0}; range.value < 2; range.value++) {
    many.push(new Second(`many-${range.value}`, {input: another.untainted}));
}
resolveLoopingOverMany(pulumi.output(many.map(v => (v.untainted.apply(untainted => untainted ? "a" : "b")))));
