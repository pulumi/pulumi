import * as pulumi from "@pulumi/pulumi";
import * as constant from "@pulumi/constant";

const first = new constant.Resource("first", {kind: "Constant"});
export const kind = first.kind;
