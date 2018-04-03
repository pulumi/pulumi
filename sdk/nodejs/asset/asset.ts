// Copyright 2017-2018, Pulumi Corporation.
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

import { Input } from "../resource";

/**
 * Asset represents a single blob of text or data that is managed as a first class entity.
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
    public readonly path: Promise<string>; // the path to the asset file.

    constructor(path: string | Promise<string>) {
        super();
        this.path = Promise.resolve(path);
    }
}

/**
 * StringAsset is a kind of asset produced from an in-memory UTF8-encoded string.
 */
export class StringAsset extends Asset {
    public readonly text: Promise<string>; // the string contents.

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
    public readonly uri: Promise<string>; // the URI where the asset lives.

    constructor(uri: string | Promise<string>) {
        super();
        this.uri = Promise.resolve(uri);
    }
}

