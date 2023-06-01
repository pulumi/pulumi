import * as pulumi from "@pulumi/pulumi";
import * as std from "@pulumi/std";

const everyArg = std.AbsMultiArgsOutput(10, 20, 30);
const onlyRequiredArgs = std.AbsMultiArgsOutput(10);
const optionalArgs = std.AbsMultiArgsOutput(10, undefined, 30);
const nestedUse = std.AbsMultiArgsOutput(everyArg, std.AbsMultiArgsOutput(42));
export const result = nestedUse;
