// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {asset} from "./decors";

// Asset represents a blob of text or data that is managed as a first class entity.
export abstract class Asset {
    constructor() {
    }
}

// Blob is a kind of asset produced from an in-memory blob represented as a byte array.
/* TODO: enable this once Uint8Array is supported.
export class Blob extends Asset {
    constructor(data: Uint8Array) {
        super();
    }
}
*/

// Code is an asset made out of an existing code artifact.  This is a compiler trick and is up to individual Lumi
// language compilers to implement.  At compile-time, the code argument must be a piece of code that can be transformed
// into a standalone asset.  The details of this complicated process are explained elsewhere.
export class Code extends Asset {
    public readonly code: any; // the code, serialized at runtime into an asset.

    constructor(code: any) {
        super();
        this.code = code;
    }
}

// File is a kind of asset produced from a given path to a file on the local filesystem.
export class File extends Asset {
    public readonly path: string; // the path to the asset file.

    constructor(path: string) {
        super();
        this.path = path;
    }
}

// String is a kind of asset produced from an in-memory UTF8-encoded string.
export class String extends Asset {
    public readonly text: string; // the string contents.

    constructor(text: string) {
        super();
        this.text = text;
    }
}

// Remote is a kind of asset produced from a given URI string.  The URI's scheme dictates the protocol for fetching
// contents: `file://` specifies a local file, `http://` and `https://` specify HTTP and HTTPS, respectively.  Note that
// specific providers may recognize alternative schemes; this is merely the base-most set that all providers support.
export class Remote extends Asset {
    public readonly uri: string; // the URI where the asset lives.

    constructor(uri: string) {
        super();
        this.uri = uri;
    }
}

