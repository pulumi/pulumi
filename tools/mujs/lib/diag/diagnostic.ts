// Copyright 2016 Marapongo, Inc. All rights reserved.

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

