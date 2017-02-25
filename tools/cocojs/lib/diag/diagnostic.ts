// Copyright 2016 Pulumi, Inc. All rights reserved.

import * as ast from "../ast";

// A diagnostic message.
export interface Diagnostic {
    category:      DiagnosticCategory; // the category of this diagnostic.
    code:          number;             // the message code.
    message:       string;             // the message text.
    loc?:          ast.Location;       // a source location covering the region of this diagnostic.
    source?:       string;             // an optional pointer to the program text, for advanced diagnostics.
    preformatted?: boolean;            // true if this message has already been formatted.
}

// A diagnostic category for the message.
export enum DiagnosticCategory {
    Warning = 0,
    Error   = 1,
}

// success returns true if the diagnostics array contains zero errors.
export function success(diags: Diagnostic[]): boolean {
    return countErrors(diags) === 0;
}

// hasErrors returns true if the diagnostics array contains at least one error.
export function hasErrors(diags: Diagnostic[]): boolean {
    return countErrors(diags) > 0;
}

// countErrors returns the number of error diagnostics in the given array, if any.
export function countErrors(diags: Diagnostic[]): number {
    let errors: number = 0;
    for (let diag of diags) {
        if (diag.category === DiagnosticCategory.Error) {
            errors++;
        }
    }
    return errors;
}

