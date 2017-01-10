// Copyright 2016 Marapongo. All rights reserved.

"use strict";

import {fs} from "nodets";
import * as fspath from "path";
import * as pack from "../pack";

const MU_PROJECT_FILE = "Mu.json";

// This function discovers the Mu metadata from a given root directory.
export async function discover(root: string): Promise<pack.Metadata> {
    // First read in the project file's contents.
    // TODO: support YAML.
    let path: string = fspath.join(root, MU_PROJECT_FILE);
    let meta: any = JSON.parse(await fs.readFile(path));
    // And now validate that it's got the correct fields, coerce it, and return a metadata object.
    if (!meta.name) {
        throw new Error(`A package name is missing from Mu project file '${path}'`);
    }
    return <pack.Metadata>meta;
}

