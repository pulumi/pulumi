// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

