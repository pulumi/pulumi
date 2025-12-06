// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import { Random } from "./random";

// Test that replacementTrigger can be set with a secret value via transforms
const res = new Random("res", { length: 7 }, {
    transforms: [
        async ({ props, opts }) => {
            opts.replacementTrigger = pulumi.secret("secret-trigger-value");
            return { props: props, opts: opts };
        },
    ],
});

