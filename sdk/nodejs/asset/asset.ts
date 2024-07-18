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

/**
 * {@link Asset} represents a single blob of text or data that is managed as a
 * first-class entity.
 */
export abstract class Asset {
    /**
     * A private field to help with RTTI that works in SxS scenarios.
     *
     * @internal
     */
    // eslint-disable-next-line @typescript-eslint/naming-convention,no-underscore-dangle,id-blacklist,id-match
    public readonly __pulumiAsset: boolean = true;

    /**
     * Returns true if the given object is an {@link Asset}. This is designed to
     * work even when multiple copies of the Pulumi SDK have been loaded into
     * the same process.
     */
    public static isInstance(obj: any): obj is Asset {
        return utils.isInstance<Asset>(obj, "__pulumiAsset");
    }
}

/**
 * {@link FileAsset} is a kind of asset produced from a given path to a file on
 * the local filesystem.
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
 * {@link StringAsset} is a kind of asset produced from an in-memory
 * UTF8-encoded string.
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
 * {@link RemoteAsset} is a kind of asset produced from a given URI string. The
 * URI's scheme dictates the protocol for fetching contents: `file://` specifies
 * a local file, `http://` and `https://` specify HTTP and HTTPS, respectively.
 * Note that specific providers may recognize alternative schemes; this is
 * merely the smallest set that all providers support.
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
