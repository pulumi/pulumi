// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
import * as pulumi from "@pulumi/pulumi";
import { Resource } from "./resource";


let config = new pulumi.Config();
export const a = new Resource("a", {
    state: {
        // Driven by table tests in steps_test.go.
        [config.require("propertyName")]: "foo",
    }
});
