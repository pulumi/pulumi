import * as pulumi from "@pulumi/pulumi";
import * as unknown from "@pulumi/unknown";

const provider = new pulumi.providers.Unknown("provider", {});
const main = new unknown.index.Main("main", {
    first: "hello",
    second: {
        foo: "bar",
    },
});
const fromModule: unknown.eks.Example[] = [];
for (let range = 0; range < 10; range++) {
    fromModule.push(new unknown.eks.Example(`fromModule-${range}`, {associatedMain: main.id}));
}
export const mainId = main.id;
export const values = fromModule.values.first;
