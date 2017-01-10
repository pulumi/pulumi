// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as ast from "../ast";
import * as symbols from "../symbols";

// A metadata section for just the informational tidbits of a package.
export interface Metadata {
    name: string;         // a required fully qualified name.
    description?: string; // an optional informational description.
    author?: string;      // an optional author.
    website?: string;     // an optional website for additional information.
    license?: string;     // an optional license governing this package's usage.
}

// A top-level package definition.
export interface Package extends Metadata {
    dependencies?: symbols.ModuleToken[]; // all of the module dependencies.
    modules?: ast.Modules;                // a collection of top-level modules.
}

