// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as pkg from "@pulumi/pkg";

export const res1 = new pkg.Random("res1", { length: 5 });

export const res2 = pkg.doEcho({ echo: "hello" });

export const res3 = pkg.doEchoOutput({ echo: "hello" });

export const res4 = pkg.doMultiEcho("hello_a", "hello_b");

export const res5 = pkg.doMultiEchoOutput("hello_a", "hello_b");

export const res6 = new pkg.Echo("echo");

export const res7 = res6.doEchoMethod({ echo: "hello" });

// Read on testprovider.Random just returns inputs back, so this works even though the resource
// doesn't exist in the state.
export const res8 = pkg.Random.get("res8", "banana");
