// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import { Input } from "../resource";
import { Asset } from "./asset";

/**
 * An Archive represents a collection of named assets.
 */
export abstract class Archive {
}

/**
 * AssetMap is a map of assets.
 */
export type AssetMap = {[name: string]: Asset | Archive};

/**
 * An AssetArchive is an archive created from an in-memory collection of named assets or other archives.
 */
export class AssetArchive extends Archive {
    public readonly assets: Promise<AssetMap>; // a map of name to asset.

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
    public readonly path: Promise<string>; // the path to the asset file.

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
    public readonly uri: Promise<string>; // the URI where the archive lives.

    constructor(uri: string | Promise<string>) {
        super();
        this.uri = Promise.resolve(uri);
    }
}

