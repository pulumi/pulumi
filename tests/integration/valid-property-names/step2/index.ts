// Copyright 2016, Pulumi Corporation.  All rights reserved.
import * as pulumi from "@pulumi/pulumi";
import { Resource } from "./resource";

let config = new pulumi.Config();
const propertyNames = config.requireObject<string[]>("propertyNames");
export const resources = propertyNames.map((_, i) => new Resource(`a${i}`, {
    state: {
        template: {
            metadata: {
                annotations: {},
            },
        },
    }
}));
