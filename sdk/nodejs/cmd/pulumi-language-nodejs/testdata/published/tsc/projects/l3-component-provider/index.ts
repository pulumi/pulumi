import * as pulumi from "@pulumi/pulumi";
import { ProviderComponent } from "./providerComponent";

const myComponent = new ProviderComponent("myComponent", {text: "hello"});
export const result = myComponent.result;
