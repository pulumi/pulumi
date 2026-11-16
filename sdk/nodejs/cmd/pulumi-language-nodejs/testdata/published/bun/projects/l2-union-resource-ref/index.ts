import * as pulumi from "@pulumi/pulumi";
import * as unionref from "@pulumi/unionref";

const x = new unionref.XType("x", {});
const y = new unionref.YType("y", {});
const z = new unionref.ZType("z", {
    value: x,
    wrapper: {
        value: y,
    },
    values: [
        x,
        y,
    ],
});
