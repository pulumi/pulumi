import * as pulumi from "@pulumi/pulumi";
import * as provider from "@pulumi/provider";

// Create a component from the generated SDK
export const comp = new provider.MyComponent("test");
