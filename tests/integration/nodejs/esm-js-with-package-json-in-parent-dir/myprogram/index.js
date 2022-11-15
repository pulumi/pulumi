// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as fs from "fs";
import assert from "node:assert";

// ensure that import.meta.url exists which is only available in ESM modules.
// This just serves as a verification that this module is actually treated as an ESM module.
assert(import.meta.url !== undefined);

export const res = fs.readFileSync("Pulumi.yaml").toString();
