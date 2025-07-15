// Copyright 2025, Pulumi Corporation.  All rights reserved.

import * as fs from "fs";
import { x } from "./other.js"; // this is the "by design" way to do this, even in TS

// Use top-level await
await new Promise(r => setTimeout(r, 2000));

// Template Literal Types were introduced in Typescript 4.1. Include a use of
// this syntax here, to validate we're using the project's TypeScript version,
// and not the vendored 3.8.3 compiler that ships with Pulumi.
type EventName<T extends string> = `on${Capitalize<T>}`;

export const res = fs.readFileSync("Pulumi.yaml").toString();
export const otherx = x;
