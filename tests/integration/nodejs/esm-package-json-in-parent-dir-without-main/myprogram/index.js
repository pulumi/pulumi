// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
// If this program runs successfully, then the test passes.
// Locating and executing this file is enough to demonstrate that the package.json
// has been read correctly, because the assert statement proves that the 
// module field has been read from the package.json located 
// in the parent directory.
import * as fs from "fs";
import assert from "node:assert";

// ensure that import.meta.url exists which is only available in ESM modules.
// This just serves as a verification that this module is actually treated as an ESM module.
assert(import.meta.url !== undefined);
