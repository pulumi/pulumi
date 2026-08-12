import * as pulumi from "@pulumi/pulumi";
import * as plaincomponent from "@pulumi/plaincomponent";

const myComponent = new plaincomponent.Component("myComponent", {
    name: "my-resource",
    settings: {
        enabled: true,
        tags: {
            env: "test",
        },
    },
});
export const label = myComponent.label;
