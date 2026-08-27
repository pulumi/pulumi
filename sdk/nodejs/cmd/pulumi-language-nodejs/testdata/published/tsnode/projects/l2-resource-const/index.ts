import * as pulumi from "@pulumi/pulumi";
import * as constant from "@pulumi/constant";

const first = new constant.Resource("first", {
    kind: "Constant",
    flag: true,
    count: 3,
    ratio: 1.5,
});
export const kind = first.kind;
export const flag = first.flag;
export const count = first.count;
export const ratio = first.ratio;
