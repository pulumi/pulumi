import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";
import { Submodule } from "./submodule";

const config = new pulumi.Config();
const listVar = config.getObject<Array<string>>("listVar") || [
    "one",
    "two",
    "three",
];
const filterCond = config.getBoolean("filterCond") || true;
const res: simple.Resource[] = [];
for (const range of Object.entries(listVar.map((v, k) => [k, v]).filter(([k, v]) => filterCond).reduce((__obj, [k, v]) => ({ ...__obj, [k]: v }))).map(([k, v]) => ({key: k, value: v}))) {
    res.push(new simple.Resource(`res-${range.key}`, {value: true}));
}
const eventualListVar = pulumi.secret(listVar);
const eventualRes: simple.Resource[] = [];
eventualListVar.apply(eventualListVar => {
    for (const range of Object.entries(eventualListVar.map((v, k) => [k, v]).filter(([k, v]) => filterCond).reduce((__obj, [k, v]) => ({ ...__obj, [k]: v }))).map(([k, v]) => ({key: k, value: v}))) {
        eventualRes.push(new simple.Resource(`eventualRes-${range.key}`, {value: true}));
    }
});
const submoduleComp = new Submodule("submoduleComp", {
    submoduleListVar: ["one"],
    submoduleFilterCond: true,
    submoduleFilterVariable: 1,
});
