// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as fs from "fs";
import { x } from "./other.js"; // this is the "by design" way to do this, even in TS
import * as process from "process";

// Use top-level await
await new Promise(r => setTimeout(r, 2000));


export const res = fs.readFileSync("Pulumi.yaml").toString();
export const otherx = x;

export const something = (arg: string): unknown => arg;

// Type error, but TSC does not type check
export const s: string = something("hello");
