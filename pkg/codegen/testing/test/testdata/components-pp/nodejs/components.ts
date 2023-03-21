import * as pulumi from "@pulumi/pulumi";
import { ExampleComponent } from "./exampleComponent";
import { SimpleComponent } from "./simpleComponent";

const simpleComponent = new SimpleComponent("simpleComponent");
const exampleComponent = new ExampleComponent("exampleComponent", {
    input: "doggo",
    ipAddress: [
        127,
        0,
        0,
        1,
    ],
    cidrBlocks: {
        one: "uno",
        two: "dos",
    },
});
export const result = exampleComponent.result;
