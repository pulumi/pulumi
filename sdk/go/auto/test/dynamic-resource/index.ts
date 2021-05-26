// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

import { Resource } from "./resource";

// Violates the first policy.
const a = new Resource("a", { state: 1 });