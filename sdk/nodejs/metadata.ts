// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

// This file exports metadata about the context in which a program is being run.

import * as runtime from "./runtime";

/**
 * getProject returns the current project name.  It throws an exception if none is registered.
 */
export function getProject(): string {
    const project = runtime.getProject();
    if (project) {
        return project;
    }
    throw new Error("Project unknown; are you using the Pulumi CLI?");
}
/**
 * getStack returns the current stack name.  It throws an exception if none is registered.
 */
export function getStack(): string {
    const stack = runtime.getStack();
    if (stack) {
        return stack;
    }
    throw new Error("Stack unknown; are you using the Pulumi CLI?");
}
