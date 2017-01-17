// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import * as yaml from "js-yaml";
import {fs} from "nodets";
import * as fspath from "path";
import * as diag from "../diag";
import * as pack from "../pack";

// projectFileBase is the base filename for Mufiles (sans extension).
const projectFileBase = "Mu";

// projectUnmarshalers is a mapping from Mufile extension to a function to unmarshal a raw blob.
let projectUnmarshalers = new Map<string, (raw: string) => any>([
    [ ".json", JSON.parse ],
    [ ".yaml", yaml.load  ],
]);

// This function discovers the Mu metadata from a given root directory.
export async function discover(root: string): Promise<DiscoverResult> {
    let dctx = new diag.Context(root);
    let diagnostics: diag.Diagnostic[] = [];

    // First read in the project file's contents, trying all available metadata formats.
    let blob: any | undefined;
    let blobPath: string | undefined;
    let base: string = fspath.join(root, projectFileBase);
    let triedExts: string[] = [];
    for (let unmarshaler of projectUnmarshalers) {
        let path: string = base + unmarshaler[0];

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

        triedExts.push(unmarshaler[0]);
    }

    let meta: pack.Metadata | undefined;
    if (blob) {
        // Ensure the project has the correct fields, coerce it, and return a metadata object.
        if (blob.name && typeof blob.name === "string") {
            meta = <pack.Metadata>blob;
        }
        else {
            diagnostics.push(dctx.newMissingMupackNameError(blobPath!));
        }
    }
    else {
        diagnostics.push(dctx.newMissingMufileError(base, triedExts));
    }

    return {
        diagnostics: diagnostics,
        meta:        meta,
    };
}

export interface DiscoverResult {
    diagnostics: diag.Diagnostic[];
    meta:        pack.Metadata | undefined;
}

