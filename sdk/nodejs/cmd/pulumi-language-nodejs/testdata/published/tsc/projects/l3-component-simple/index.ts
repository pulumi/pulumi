import * as pulumi from "@pulumi/pulumi";
import { MyComponent } from "./myComponent";

const someComponent = new MyComponent("someComponent", {input: true});
export const result = someComponent.output;
