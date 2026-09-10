import * as pulumi from "@pulumi/pulumi";

const names = [
    "alpha",
    "beta",
    "gamma",
];
const tags = {
    Environment: "production",
    Team: "infra",
};
export const prefixed = names.map(n => (`prefix-${n}`));
export const filtered = names.filter(n => n != "beta").map(n => (n));
export const indexed = names.map((v, k) => [k, v] as const).map(([i, n]) => (`${i}:${n}`));
export const tagList = Object.entries(tags).sort().map(([k, v]) => (`${k}=${v}`));
export const prefixedMap = names.reduce((__obj, n) => ({ ...__obj, [n]: `prefix-${n}` }), {});
export const filteredTags = Object.entries(tags).sort().filter(([k, v]) => k != "Team").reduce((__obj, [k, v]) => ({ ...__obj, [k]: v }), {});
