// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as ast from "../ast";
import * as tokens from "../tokens";

// Manifest is the "metadata-only" section of a package's definition file.  This part is shared between already compiled
// packages loaded as dependencies in addition to packages that are actively being compiled (and hence possibly missing
// the other parts in the full blown Package interface).
export interface Manifest {
    name: tokens.PackageToken;           // a required fully qualified name.
    description?: string;                // an optional informational description.
    author?: string;                     // an optional author.
    website?: string;                    // an optional website for additional information.
    license?: string;                    // an optional license governing this package's usage.
}

// Package is a fully compiled package definition.
export interface Package extends Manifest {
    dependencies?: tokens.ModuleToken[]; // all of the module dependencies.
    modules?: ast.Modules;               // a collection of top-level modules.
}

