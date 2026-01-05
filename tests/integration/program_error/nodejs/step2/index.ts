// Copyright 2025, Pulumi Corporation.  All rights reserved.

import { Random, Component } from "./random";

const res1 = new Random("res1", { length: 10 });

throw new Error("This is a test error");

const res2 = new Random("res2", { length: 10 });
