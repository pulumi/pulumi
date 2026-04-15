// Copyright 2016, Pulumi Corporation.  All rights reserved.

import * as fs from "fs";

export const res = fs.readFileSync("Pulumi.yaml").toString();
