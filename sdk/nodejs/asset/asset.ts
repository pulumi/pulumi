// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import { Computed, ComputedValue } from "../resource";

/**
 * Asset represents a blob of text or data that is managed as a first class entity.
 */
export abstract class Asset {
}

/**
 * Blob is a kind of asset produced from an in-memory blob represented as a byte array.
 */
/* IDEA: enable this once Uint8Array is supported.
export class Blob extends Asset {
    constructor(data: Uint8Array) {
        super();
    }
}
*/

/**
 * FileAsset is a kind of asset produced from a given path to a file on the local filesystem.
 */
export class FileAsset extends Asset {
    public readonly path: Computed<string>; // the path to the asset file.

    constructor(path: ComputedValue<string>) {
        super();
        this.path = Promise.resolve<string | undefined>(path);
    }
}

/**
 * StringAsset is a kind of asset produced from an in-memory UTF8-encoded string.
 */
export class StringAsset extends Asset {
    public readonly text: Computed<string>; // the string contents.

    constructor(text: ComputedValue<string>) {
        super();
        this.text = Promise.resolve<string | undefined>(text);
    }
}

/**
 * RemoteAsset is a kind of asset produced from a given URI string.  The URI's scheme dictates the protocol for fetching
 * contents: `file://` specifies a local file, `http://` and `https://` specify HTTP and HTTPS, respectively.  Note that
 * specific providers may recognize alternative schemes; this is merely the base-most set that all providers support.
 */
export class RemoteAsset extends Asset {
    public readonly uri: Computed<string>; // the URI where the asset lives.

    constructor(uri: ComputedValue<string>) {
        super();
        this.uri = Promise.resolve<string | undefined>(uri);
    }
}

