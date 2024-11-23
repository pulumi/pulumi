import * as pulumi from "@pulumi/pulumi";
import { First } from "./first";
import { Second } from "./second";

const [secondPasswordLength, resolveSecondPasswordLength] = pulumi.deferredOutput<any>();
const first = new First("first", {passwordLength: secondPasswordLength});
const second = new Second("second", {petName: first.petName});
resolveSecondPasswordLength(second.passwordLength);
const [loopingOverMany, resolveLoopingOverMany] = pulumi.deferredOutput<Array<any>>();
const another = new First("another", {passwordLength: loopingOverMany.length});
const many: Second[] = [];
for (const range = {value: 0}; range.value < 10; range.value++) {
    many.push(new Second(`many-${range.value}`, {petName: another.petName}));
}
resolveLoopingOverMany(many.map((v, k) => [k, v]).map(([_, v]) => (v.passwordLength)));
