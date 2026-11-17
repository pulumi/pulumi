import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const a = config.requireNumber("a");
const b = config.requireNumber("b");
const c = config.requireNumber("c");
const d = config.requireNumber("d");
export const maxResult = Math.max(a, b);
export const minResult = Math.min(a, b);
export const intMaxResult = Math.max(c, d);
export const intMinResult = Math.min(c, d);
