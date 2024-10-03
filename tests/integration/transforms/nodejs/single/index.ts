// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import { Random } from "./random";

pulumi.runtime.registerStackTransform(async ({ type, props, opts }) => {
    console.log("stack transform");
    return undefined;
});

new Random("res1", { length: pulumi.secret(5) });