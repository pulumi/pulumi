// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
import { Resource } from "./resource";

export const a = new Resource("a", {
    state: {
        template: {
            metadata: {
                annotations: {},
            },
        },
    }
});
