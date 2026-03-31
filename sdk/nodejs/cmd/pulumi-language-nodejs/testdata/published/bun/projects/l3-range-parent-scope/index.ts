import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const config = new pulumi.Config();
const prefix = config.require("prefix");
const item: nestedobject.Target[] = [];
for (const range = {value: 0}; range.value < 2; range.value++) {
    item.push(new nestedobject.Target(`item-${range.value}`, {name: `${prefix}-${range.value}`}));
}
