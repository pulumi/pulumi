import * as pulumi from "@pulumi/pulumi";

export const expandedMax = Math.max(...[
    1,
    2,
    3,
]);
export const expandedMaxWithPrefix = Math.max(0, ...[
    1,
    2,
    3,
]);
