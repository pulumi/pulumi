import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const config = new pulumi.Config();
const extraCount = config.requireNumber("extraCount");
const res1 = new simple.Resource("res1", {value: true});
const res2: simple.Resource[] = [];
for (let range = 0; range < extraCount; range++) {
    res2.push(new simple.Resource(`res2-${range}`, {value: false}));
}
