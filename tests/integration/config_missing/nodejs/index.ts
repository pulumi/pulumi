// Copyright 2016, Pulumi Corporation.  All rights reserved.

import { Config } from "@pulumi/pulumi";

const config = new Config("config_missing_js");
config.requireSecret("notFound")