// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

// Motivating case: an AND reducer where `true` sticks to `false`. Once the reduced output
// becomes `false`, it can never flip back to `true` even if the program's input is `true`.
const stash = new pulumi.Stash("bucket", {
    input: true,
    reduce: (_oldInput, oldOutput, newInput) => oldOutput && newInput,
});

export const output = stash.output;
export const input = stash.input;
