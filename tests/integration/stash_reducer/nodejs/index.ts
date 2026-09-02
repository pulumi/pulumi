// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

// Motivating case: an AND reducer where `true` sticks to `false`. Once the reduced output
// becomes `false`, it can never flip back to `true` even if the program's input is `true`.
// On create the reducer runs with `oldOutput === null`; we treat that as the identity for &&.
const reduce: pulumi.StashReducer = (_oldInput, oldOutput, newInput) =>
    oldOutput === null || oldOutput === undefined ? newInput : oldOutput && newInput;

const stash = new pulumi.Stash("bucket", { input: true, reduce });

export const output = stash.output;
export const input = stash.input;
