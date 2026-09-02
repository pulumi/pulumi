// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

// Once the reduced output is `false`, flipping input back to `true` must not resurrect it:
// reducer(true, false, true) === false && true === false.
const reduce: pulumi.StashReducer = (_oldInput, oldOutput, newInput) =>
    oldOutput === null || oldOutput === undefined ? newInput : oldOutput && newInput;

const stash = new pulumi.Stash("bucket", { input: true, reduce });

export const output = stash.output;
export const input = stash.input;
