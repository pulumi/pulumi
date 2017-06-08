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

/* tslint:disable:no-empty */
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

