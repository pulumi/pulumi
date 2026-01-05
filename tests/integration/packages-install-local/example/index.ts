import * as pulumi from "@pulumi/pulumi";
import * as provider from "@pulumi/provider";

export const comp = new provider.MyComponent("test")
