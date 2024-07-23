// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as pkg from "@pulumi/pkg";

const res1 = new pkg.Random("res1", { length: 5 });