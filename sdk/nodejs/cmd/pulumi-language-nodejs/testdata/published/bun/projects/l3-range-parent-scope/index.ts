import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const config = new pulumi.Config();
const prefix = config.require("prefix");
const item: nestedobject.Target[] = [];
for (let range = {value: 0}; range.value < 2; range = {value: range.value + 1}) {
    item.push(new nestedobject.Target(`item-${range.value}`, {name: `${prefix}-${range.value}`}));
}
