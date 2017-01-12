// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import {fs} from "nodets";
import * as fspath from "path";
import * as diag from "../diag";
import * as pack from "../pack";

const MU_PROJECT_FILE = "Mu.json";

// This function discovers the Mu metadata from a given root directory.
export async function discover(root: string): Promise<DiscoverResult> {
    let dctx = new diag.Context(root);
    let diagnostics: diag.Diagnostic[] = [];

    // TODO: support YAML.
    let path: string = fspath.join(root, MU_PROJECT_FILE);

    // First read in the project file's contents.
    let blob: any | undefined;
    try {
        blob = JSON.parse(await fs.readFile(path));
    }
    catch (err) {
        if (err.code !== "ENOENT") {
            // For anything but "missing file" errors, rethrow the error.
            throw err;
        }
        diagnostics.push(dctx.newMissingMufileError(path));
    }

    let meta: pack.Metadata | undefined;
    if (blob) {
        // Now validate that it's got the correct fields, coerce it, and return a metadata object.
        if (blob.name && typeof blob.name === "string") {
            meta = <pack.Metadata>blob;
        }
        else {
            diagnostics.push(dctx.newMissingMupackNameError(path));
        }
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

