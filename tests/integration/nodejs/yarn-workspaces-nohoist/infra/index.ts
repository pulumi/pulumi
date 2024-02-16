// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//
// If this program runs successfully, then the test passes.
// Executing this file demonstrates that we're able to successfully install
// dependencies and run in a yarn workspaces setup. 

import * as myRandom from "my-random";

const random = new myRandom.MyRandom("plop", {})

export const id = random.randomID
