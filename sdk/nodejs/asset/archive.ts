// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { Computed, MaybeComputed } from "../computed";
import { Property } from "../runtime";
import { Asset } from "./asset";

// An Archive represents a collection of named assets.
export abstract class Archive {
}

// AssetMap is a map of assets.
export type AssetMap = {[name: string]: Asset};

// An AssetArchive is an archive created with a collection of named assets.
export class AssetArchive extends Archive {
    public readonly assets: Computed<AssetMap>; // a map of name to asset.

    constructor(assets: MaybeComputed<AssetMap>) {
        super();
        this.assets = new Property<AssetMap>(assets);
    }
}

// A FileArchive is an archive in a file-based archive in one of the supported formats (.tar, .tar.gz, or .zip).
export class FileArchive extends Archive {
    public readonly path: Computed<string>; // the path to the asset file.

    constructor(path: MaybeComputed<string>) {
        super();
        this.path = new Property<string>(path);
    }
}

// A RemoteArchive is an archive in a file-based archive fetched from a remote location.  The URI's scheme dictates the
// protocol for fetching the archive's contents: `file://` is a local file (just like a FileArchive), `http://` and
// `https://` specify HTTP and HTTPS, respectively, and specific providers may recognize custom schemes.
export class RemoteArchive extends Archive {
    public readonly uri: Computed<string>; // the URI where the archive lives.

    constructor(uri: MaybeComputed<string>) {
        super();
        this.uri = new Property<string>(uri);
    }
}

