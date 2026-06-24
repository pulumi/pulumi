import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const config = new pulumi.Config();
const numItems = config.requireNumber("numItems");
const itemList = config.requireObject<Array<string>>("itemList");
const itemMap = config.requireObject<Record<string, string>>("itemMap");
const createBool = config.requireBoolean("createBool");
const numResource: nestedobject.Target[] = [];
for (let range = {value: 0}; range.value < numItems; range = {value: range.value + 1}) {
    numResource.push(new nestedobject.Target(`numResource-${range.value}`, {name: `num-${range.value}`}));
}
const listResource: nestedobject.Target[] = [];
for (const range of itemList.map((v, k) => ({key: k, value: v}))) {
    listResource.push(new nestedobject.Target(`listResource-${range.key}`, {name: `${range.key}:${range.value}`}));
}
const mapResource: {[key: string]: nestedobject.Target} = {};
for (const range of Object.entries(itemMap).sort().map(([k, v]) => ({key: k, value: v}))) {
    mapResource[range.key] = new nestedobject.Target(`mapResource-${range.key}`, {name: `${range.key}=${range.value}`});
}
let boolResource: nestedobject.Target | undefined;
if (createBool) {
    boolResource = new nestedobject.Target("boolResource", {name: "bool-resource"});
}
