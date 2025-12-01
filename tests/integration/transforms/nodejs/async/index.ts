// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import { Random } from "./random";

setTimeout(() => {
    const res = new Random("res", { length: 10 }, {
        transforms: [
            async ({ type, props, opts }) => {
                console.log("res transform");
                return {
                    props: { ...props, length: 12 },
                    opts: opts,
                };
            },
        ],
    })
})
