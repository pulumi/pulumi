// Copyright 2016 Pulumi, Inc. All rights reserved.

import * as yaml from "js-yaml";
import * as ast from "../ast";
import * as tokens from "../tokens";

// cocofileBase is the base filename for Cocofiles (sans extension).
export const cocofileBase = "Coconut";

// cocopackBase is the base filename for CocoPackages (sans extension).
export const cocopackBase = "Cocopack";

// defaultFormatExtension is the default extension used for the Cocofile/CocoPackage formats.
export const defaultFormatExtension = ".json";

// marshalers is a mapping from Cocofile/CocoPackage format extension to a function to marshal an object into a string.
export let marshalers = new Map<string, (obj: any) => string>([
    [ ".json", (obj: any) => JSON.stringify(obj, null, 4) ],
    [ ".yaml", yaml.dump ],
]);

// unmarshalers is a mapping from Cocofile/CocoPackage format extension to a function to unmarshal a raw string blob.
export let unmarshalers = new Map<string, (raw: string) => any>([
    [ ".json", JSON.parse ],
    [ ".yaml", yaml.load  ],
]);

// Manifest is the "metadata-only" section of a package's definition file.  This part is shared between already compiled
// packages loaded as dependencies in addition to packages that are actively being compiled (and hence possibly missing
// the other parts in the full blown Package interface).
export interface Manifest {
    name: tokens.PackageToken;   // a required fully qualified name.
    description?: string;        // an optional informational description.
    author?: string;             // an optional author.
    website?: string;            // an optional website for additional information.
    license?: string;            // an optional license governing this package's usage.
    dependencies?: Dependencies; // all of the package's dependencies.
}

// Dependencies is a map from dependency package token to a version string.
export type Dependencies = {[pkg: string/*tokens.PackageToken*/]: string};

// Package is a fully compiled package definition.
export interface Package extends Manifest {
    modules?: ast.Modules;   // a collection of top-level modules.
    aliases?: ModuleAliases; // an optional map of aliased module names.
}

// ModuleAliases can be used to map module names to other module names during binding.  This is useful for representing
// "default" modules in various forms; e.g., "index" as ".default"; "lib/index" as "lib"; and so on.
export type ModuleAliases = {[name: string/*tokens.ModuleName*/]: string/*tokens.ModuleName*/};

