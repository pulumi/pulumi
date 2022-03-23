// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as fs from "fs";

export const res = fs.readFileSync("Pulumi.yaml").toString();
