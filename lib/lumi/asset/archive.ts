// Copyright 2017 Pulumi, Inc. All rights reserved.

import {Asset} from "./asset";

// An Archive represents a collection of named assets.
export abstract class Archive {
    constructor() {
    }
}

// An AssetArchive is an archive created with a collection of named assets.
export class AssetArchive extends Archive {
    public readonly assets: {[name: string]: Asset}; // a map of name to asset.

    constructor(assets: {[name: string]: Asset}) {
        super();
        this.assets = assets;
    }
}

// A FileArchive is an archive in a file-based archive in one of the supported formats (.tar, .tar.gz, or .zip).
export class FileArchive extends Archive {
    public readonly path: string; // the path to the asset file.

    constructor(path: string) {
        super();
        this.path = path;
    }
}

// A RemoteArchive is an archive in a file-based archive fetched from a remote location.  The URI's scheme dictates the
// protocol for fetching the archive's contents: `file://` is a local file (just like a FileArchive), `http://` and
// `https://` specify HTTP and HTTPS, respectively, and specific providers may recognize custom schemes.
export class RemoteArchive extends Archive {
    public readonly uri: string; // the URI where the archive lives.

    constructor(uri: string) {
        super();
        this.uri = uri;
    }
}

