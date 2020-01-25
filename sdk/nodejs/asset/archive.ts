// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as utils from "../utils";
import { Asset } from "./asset";

/**
 * An Archive represents a collection of named assets.
 */
export abstract class Archive {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     * @internal
     */
    // tslint:disable-next-line:variable-name
    public readonly __pulumiArchive: boolean = true;

    /**
     * Returns true if the given object is an instance of an Archive.  This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is Archive {
        return utils.isInstance<Archive>(obj, "__pulumiArchive");
    }
}

/**
 * AssetMap is a map of assets.
 */
export type AssetMap = {[name: string]: Asset | Archive};

/**
 * An AssetArchive is an archive created from an in-memory collection of named assets or other archives.
 */
export class AssetArchive extends Archive {
    /**
     * A map of names to assets.
     */
    public readonly assets: Promise<AssetMap>;

    constructor(assets: AssetMap | Promise<AssetMap>) {
        super();
        this.assets = Promise.resolve(assets);
    }
}

/**
 * A FileArchive is a file-based archive, or a collection of file-based assets.  This can be a raw directory or a
 * single archive file in one of the supported formats (.tar, .tar.gz, or .zip).
 */
export class FileArchive extends Archive {
    /**
     * The path to the asset file.
     */
    public readonly path: Promise<string>;

    constructor(path: string | Promise<string>) {
        super();
        this.path = Promise.resolve(path);
    }
}

/**
 * A RemoteArchive is a file-based archive fetched from a remote location.  The URI's scheme dictates the
 * protocol for fetching the archive's contents: `file://` is a local file (just like a FileArchive), `http://` and
 * `https://` specify HTTP and HTTPS, respectively, and specific providers may recognize custom schemes.
 */
export class RemoteArchive extends Archive {
    /**
     * The URI where the archive lives.
     */
    public readonly uri: Promise<string>;

    constructor(uri: string | Promise<string>) {
        super();
        this.uri = Promise.resolve(uri);
    }
}

