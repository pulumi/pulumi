// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

let config = new pulumi.Config();
let org = config.require("org");
let slug = `${org}/${pulumi.getProject()}/${pulumi.getStack()}`;
let a = new pulumi.StackReference(slug);

let gotError = false;
try
{
    a.getOutputSync("val2");
}
catch (err)
{
    gotError = true;
}

if (!gotError) {
    throw new Error("Expected to get error trying to read secret from stack reference.");
}