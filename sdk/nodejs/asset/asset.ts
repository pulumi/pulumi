// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import { Input } from "../resource";

/**
 * Asset represents a single blob of text or data that is managed as a first class entity.
 */
export abstract class Asset {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     */
    // tslint:disable-next-line:variable-name
    private readonly __pulumiAsset: boolean = true;

    /**
     * Returns true if the given object is an instance of an Asset.  This is designed to work even when
     * multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is Asset {
        return obj && obj.__pulumiAsset;
    }
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
 * StringAsset is a kind of asset produced from an in-memory UTF8-encoded string.
 */
export class StringAsset extends Asset {
    /**
     * The string contents.
     */
    public readonly text: Promise<string>;

    constructor(text: string | Promise<string>) {
        super();
        this.text = Promise.resolve(text);
    }
}

/**
 * RemoteAsset is a kind of asset produced from a given URI string.  The URI's scheme dictates the protocol for fetching
 * contents: `file://` specifies a local file, `http://` and `https://` specify HTTP and HTTPS, respectively.  Note that
 * specific providers may recognize alternative schemes; this is merely the base-most set that all providers support.
 */
export class RemoteAsset extends Asset {
    /**
     * The URI where the asset lives.
     */
    public readonly uri: Promise<string>;

    constructor(uri: string | Promise<string>) {
        super();
        this.uri = Promise.resolve(uri);
    }
}

