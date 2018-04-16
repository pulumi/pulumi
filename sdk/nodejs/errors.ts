// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

/**
 * RunError can be used for terminating a program abruptly, but resulting in a clean exit rather than the usual
 * verbose unhandled error logic which emits the source program text and complete stack trace.
 */
export class RunError extends Error {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     */
    // tslint:disable-next-line:variable-name
    private readonly __pulumiRunError: boolean = true;

    /**
     * Returns true if the given object is an instance of a RunError.  This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is RunError {
        return obj && obj.__pulumiRunError;
    }

    constructor(message: string) {
        super(message);
    }
}

