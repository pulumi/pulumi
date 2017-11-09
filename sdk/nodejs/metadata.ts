// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// This file exports metadata about the context in which a program is being run.

import * as runtime from "./runtime";

/**
 * getProject returns the current project name, or the empty string if there is none.
 */
export function getProject(): string {
    return runtime.options.project || "";
}
/**
 * getStack returns the current stack name, or the empty string if there is none.
 */
export function getStack(): string {
    return runtime.options.stack || "";
}
