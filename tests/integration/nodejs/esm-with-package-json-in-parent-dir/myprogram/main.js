// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
// If this program runs successfully, then the test passes.
// Locating and executing this file is enough to demonstrate that the `main` field
// has been read correctly, because the package.json file
// in the parent directory will inform Pulumi to look for main.ts
// as the entrypoint instead of main.js.

import * as fs from "fs";
import assert from "node:assert";

// ensure that import.meta.url exists which is only available in ESM modules.
// This just serves as a verification that this module is actually treated as an ESM module.
assert(import.meta.url !== undefined);
