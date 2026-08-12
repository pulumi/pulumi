import * as pulumi from "@pulumi/pulumi";
import * as nestedobject from "@pulumi/nestedobject";

const config = new pulumi.Config();
const prefix = config.require("prefix");
const item: nestedobject.Target[] = [];
for (let range = 0; range < 2; range++) {
    item.push(new nestedobject.Target(`item-${range}`, {name: `${prefix}-${range}`}));
}
