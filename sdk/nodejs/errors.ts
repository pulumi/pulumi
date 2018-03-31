// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

/**
 * RunError can be used for terminating a program abruptly, but resulting in a clean exit rather than the usual
 * verbose unhandled error logic which emits the source program text and complete stack trace.
 */
export class RunError extends Error {
    constructor(public readonly urn: string, message: string) {
        super(message);
    }
}

