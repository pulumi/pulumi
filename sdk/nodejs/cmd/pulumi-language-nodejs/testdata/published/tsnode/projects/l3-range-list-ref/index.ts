import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const config = new pulumi.Config();
const numItems = config.requireNumber("numItems");
const itemList = config.requireObject<Array<string>>("itemList");
const numResource: nestedobject.Target[] = [];
for (let range = 0; range < numItems; range++) {
    numResource.push(new nestedobject.Target(`numResource-${range}`, {name: `num-${range}`}));
}
const numTarget = new nestedobject.Target("numTarget", {name: pulumi.interpolate`${numResource[0].name}+`});
const listResource: nestedobject.Target[] = [];
for (const range of itemList.map((v, k) => ({key: k, value: v}))) {
    listResource.push(new nestedobject.Target(`listResource-${range.key}`, {name: `${range.key}:${range.value}`}));
}
const listTarget = new nestedobject.Target("listTarget", {name: pulumi.interpolate`${listResource[1].name}+`});
const listDynTarget: nestedobject.Target[] = [];
for (const range of itemList.map((v, k) => ({key: k, value: v}))) {
    listDynTarget.push(new nestedobject.Target(`listDynTarget-${range.key}`, {name: pulumi.interpolate`${listResource[range.key].name}!`}));
}
