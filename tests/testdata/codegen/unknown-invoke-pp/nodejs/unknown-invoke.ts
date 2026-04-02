import * as pulumi from "@pulumi/pulumi";
import * as unknown from "@pulumi/unknown";

const data = unknown.getData({
    input: "hello",
});
const values = unknown.eks.moduleValues({});
export const content = data.content;
