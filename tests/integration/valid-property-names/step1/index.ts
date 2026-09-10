// Copyright 2016, Pulumi Corporation.  All rights reserved.
import * as pulumi from "@pulumi/pulumi";
import { Resource } from "./resource";

let config = new pulumi.Config();
// Driven by table tests in steps_test.go.
const propertyNames = config.requireObject<string[]>("propertyNames");
export const resources = propertyNames.map((name, i) => new Resource(`a${i}`, {
    state: {
        [name]: "foo",
    }
}));
