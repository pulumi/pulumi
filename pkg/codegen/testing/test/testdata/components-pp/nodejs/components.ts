import * as pulumi from "@pulumi/pulumi";
import { ExampleComponent } from "./exampleComponent";
import { SimpleComponent } from "./simpleComponent";

const simpleComponent = new SimpleComponent("simpleComponent");
const exampleComponent = new ExampleComponent("exampleComponent", {input: "doggo"});
export const result = exampleComponent.result;
