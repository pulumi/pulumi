// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Asset represents a blob of text or data that is managed as a first class entity.
export abstract class Asset {
}

// Blob is a kind of asset produced from an in-memory blob represented as a byte array.
/* IDEA: enable this once Uint8Array is supported.
export class Blob extends Asset {
    constructor(data: Uint8Array) {
        super();
    }
}
*/

// FileAsset is a kind of asset produced from a given path to a file on the local filesystem.
export class FileAsset extends Asset {
    public readonly path: string; // the path to the asset file.

    constructor(path: string) {
        super();
        this.path = path;
    }
}

// StringAsset is a kind of asset produced from an in-memory UTF8-encoded string.
export class StringAsset extends Asset {
    public readonly text: string; // the string contents.

    constructor(text: string) {
        super();
        this.text = text;
    }
}

// RemoteAsset is a kind of asset produced from a given URI string.  The URI's scheme dictates the protocol for fetching
// contents: `file://` specifies a local file, `http://` and `https://` specify HTTP and HTTPS, respectively.  Note that
// specific providers may recognize alternative schemes; this is merely the base-most set that all providers support.
export class RemoteAsset extends Asset {
    public readonly uri: string; // the URI where the asset lives.

    constructor(uri: string) {
        super();
        this.uri = uri;
    }
}

