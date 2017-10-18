// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { Config } from "pulumi";

let config = new Config("minimal:config");
console.log(`Hello, ${config.require("name")}!`);
console.log(`Psst, ${config.require("secret")}`);

