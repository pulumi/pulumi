import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";
import { MyComponent } from "./myComponent";

const input = new simple.Resource("input", {value: true});
const someComponent = new MyComponent("someComponent", {input: input.value});
export const result = someComponent.output;
