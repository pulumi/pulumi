// Copyright 2016, Pulumi Corporation.  All rights reserved.

import { Config } from "@pulumi/pulumi";

const config = new Config();
export const testVar = config.require("test");
