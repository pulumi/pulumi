// Copyright 2016, Pulumi Corporation.  All rights reserved.

import * as fs from "fs";

// Use top-level await
await new Promise(r => setTimeout(r, 2000));

export const res = fs.readFileSync("Pulumi.yaml").toString();
