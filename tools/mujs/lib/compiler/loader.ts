// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import * as yaml from "js-yaml";
import {contract, fs} from "nodejs-ts";
import * as fspath from "path";
import * as diag from "../diag";
import * as pack from "../pack";

// PackageLoader understands how to load a package.
export class PackageLoader {
    private cache: Map<string, pack.Manifest>; // a cache of loaded packages.

    constructor() {
        this.cache = new Map<string, pack.Manifest>();
    }

    // This function searches for Mu metadata from a given root directory.  If the upwards argument is true, it will
    // search upwards in the directory hierarchy until it finds a package file or hits the root of the filesystem.
    public async load(root: string, upwards?: boolean): Promise<PackageResult> {
        let dctx = new diag.Context(root);
        let diagnostics: diag.Diagnostic[] = [];
        let pkg: pack.Manifest | undefined;

        // First read in the project file's contents, trying all available metadata formats.
        let blob: any | undefined;
        let blobPath: string | undefined;
        let search: string = fspath.resolve(root);
        while (!pkg && !blob) {
            let base: string = fspath.join(search, pack.mufileBase);
            for (let unmarshaler of pack.unmarshalers) {
                let path: string = base + unmarshaler[0];

                // First, see if we have this package already in our cache.
                if (pkg = this.cache.get(path)) {
                    break;
                }

                // If not, try to load it from the disk.
                let raw: string | undefined;
                try {
                    raw = await fs.readFile(path);
                }
                catch (err) {
                    if (err.code !== "ENOENT") {
                        // For anything but "missing file" errors, rethrow the error.
                        throw err;
                    }
                }
                if (raw) {
                    // A file was found; parse its raw contents into a JSON object.
                    blobPath = path;
                    try {
                        blob = unmarshaler[1](raw);
                    }
                    catch (err) {
                        diagnostics.push(dctx.newMalformedMufileError(path, err));
                    }
                    break;
                }
            }

            if (!pkg && !blob) {
                // If we didn't find anything, and upwards is true, search the parent directory.
                if (upwards) {
                    if (search === "/") {
                        // If we're at the root of the filesystem, no more searching can be done.
                        search = root;
                        break;
                    }
                    search = fspath.dirname(search);
                }
                else {
                    break;
                }
            }
        }

        if (!pkg) {
            if (blob) {
                contract.assert(!!blobPath);

                // Ensure the project has the correct fields, coerce it, and return a metadata object.
                if (blob.name && typeof blob.name === "string") {
                    pkg = <pack.Package>blob;
                }
                else {
                    diagnostics.push(dctx.newMissingMupackNameError(blobPath!));
                }

                // Memoize the result so we don't need to continuously search for the same packages.
                this.cache.set(blobPath!, pkg);
            }
            else {
                // The file was missing; issue an error, and make sure to include the set of extensions we tried.
                let triedExts: string[] = [];
                for (let unmarshaler of pack.unmarshalers) {
                    triedExts.push(unmarshaler[0]);
                }
                diagnostics.push(dctx.newMissingMufileError(fspath.join(root, pack.mufileBase), triedExts));
            }
        }

        return {
            root:        search,
            pkg:         pkg,
            diagnostics: diagnostics,
        };
    }
}

export interface PackageResult {
    root:        string;                    // the root directory under which the manifest was located.
    pkg:         pack.Manifest | undefined; // the loaded package manifest (or undefined if discovery failed).
    diagnostics: diag.Diagnostic[];         // a collection of diagnostics emitted during package loading (if any).
}

