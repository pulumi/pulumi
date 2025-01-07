import * as pulumi from "@pulumi/pulumi";

// Since the name is "this" it will fail in typescript and other languages with
// this reservered keyword if it is not renamed.
const _this = "somestring";
export const output = _this;
