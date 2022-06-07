// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as fs from "fs";
import { x } from "./other.js"; // this is the "by design" way to do this, even in TS.

// Use top-level await
await new Promise(r => setTimeout(r, 2000));

export const res = fs.readFileSync("Pulumi.yaml").toString();
export const otherx = x;
