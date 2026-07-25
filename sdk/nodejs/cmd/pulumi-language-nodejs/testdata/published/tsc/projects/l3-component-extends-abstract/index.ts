import * as pulumi from "@pulumi/pulumi";
import * as inheritabstract from "@pulumi/inheritabstract";

const child = new inheritabstract.ConcreteChild("child", {
    seed: "s",
    extra: "e",
});
export const abstractOutput = child.abstractOutput;
export const concreteOutput = child.concreteOutput;
