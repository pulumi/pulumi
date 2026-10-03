import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const from = config.requireNumber("from");
const to = config.requireNumber("to");
export const rangeTo = Array.from({length: to}, (_, i) => i);
export const rangeFromTo = ((from, to) => Array.from({length: to - from}, (_, i) => from + i))(from, to);
export const literalTo = Array.from({length: 3}, (_, i) => i);
export const literalFromTo = Array.from({length: 3}, (_, i) => 2 + i);
export const literalEmpty = Array.from({length: 0}, (_, i) => 3 + i);
