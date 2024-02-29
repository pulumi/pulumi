// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import { Random } from "testprovider";
import { RandomInteger } from "random";

const a = new Random("a", 10);
const b = new RandomInteger("b", {min: 0, max: 10});