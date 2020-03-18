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

import * as pulumi from "../";
import { getRemoteServer } from "./remoteServer";

/**
 * ProxyComponentResource is the abstract base class for proxies around component resources.
 */
export abstract class ProxyComponentResource extends pulumi.ComponentResource {
    constructor(
        t: string,
        name: string,
        libraryPath: string,
        libraryName: string,
        inputs: pulumi.Inputs,
        outputs: Record<string, undefined>,
        opts: pulumi.ComponentResourceOptions = {}) {
        // There are two cases:
        // 1. A URN was provided - in this case we are just going to look up the existing resource
        //    and populate this proxy from that URN.
        // 2. A URN was not provided - in this case we are going to remotely construct the resource,
        //    get the URN from the newly constructed resource, then look it up and populate this
        //    proxy from that URN.
        if (!opts.urn) {
            const p = getRemoteServer().construct(libraryPath, libraryName, name, inputs, opts);
            const urn = p.then(r => <string>r.urn);
            opts = pulumi.mergeOptions(opts, { urn });
        }
        const props = {
            ...inputs,
            ...outputs,
        };
        super(t, name, props, opts);
    }
}
