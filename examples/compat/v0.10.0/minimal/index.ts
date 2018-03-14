// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { Config } from "@pulumi/pulumi";

let config = new Config("minimal:config");
console.log(`Hello, ${config.require("name")}!`);
console.log(`Psst, ${config.require("secret")}`);

