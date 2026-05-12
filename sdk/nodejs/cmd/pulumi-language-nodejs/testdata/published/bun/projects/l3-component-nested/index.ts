import * as pulumi from "@pulumi/pulumi";
import { OuterComponent } from "./outerComponent";

const outerComponent = new OuterComponent("outerComponent", {input: true});
export const result = outerComponent.output;
