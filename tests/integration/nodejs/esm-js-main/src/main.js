// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as fs from "fs";
import * as aws from "@pulumi/aws";

const b = new aws.s3.Bucket("b");

export const res = fs.readFileSync("Pulumi.yaml").toString();
