import * as pulumi from "@pulumi/pulumi";
import { AnotherComponent } from "./another-component";
import { ExampleComponent } from "./exampleComponent";
import { SimpleComponent } from "./simpleComponent";

const simpleComponent = new SimpleComponent("simpleComponent");
const multipleSimpleComponents: SimpleComponent[] = [];
for (const range = {value: 0}; range.value < 10; range.value++) {
    multipleSimpleComponents.push(new SimpleComponent(`multipleSimpleComponents-${range.value}`));
}
const anotherComponent = new AnotherComponent("anotherComponent");
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
    githubApp: {
        id: "example id",
        keyBase64: "base64 encoded key",
        webhookSecret: "very important secret",
    },
    servers: [
        {
            name: "First",
        },
        {
            name: "Second",
        },
    ],
    deploymentZones: {
        first: {
            zone: "First zone",
        },
        second: {
            zone: "Second zone",
        },
    },
});
export const result = exampleComponent.result;
