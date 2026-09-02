// Copyright 2026, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

// The reducer runs with (oldInput=true, oldOutput=true, newInput=false) and produces
// `true && false === false`, so the stash output should transition from true to false.
const reduce: pulumi.StashReducer = (_oldInput, oldOutput, newInput) =>
    oldOutput === null || oldOutput === undefined ? newInput : oldOutput && newInput;

const stash = new pulumi.Stash("bucket", { input: false, reduce });

export const output = stash.output;
export const input = stash.input;
