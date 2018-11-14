// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

let config = new pulumi.Config();
let org = config.get("org");
let slug = org ? `${org}/${pulumi.getStack()}` : pulumi.getStack();
let a = new pulumi.StackReference(slug);
