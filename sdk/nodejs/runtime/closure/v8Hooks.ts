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

// Module that hooks into v8 and provides information about it to interested parties. Because this
// hooks into v8 events it is critical that this module is loaded early when the process starts.
// Otherwise, information may not be known when needed.  This module is only intended for use on
// Node v11 and higher.

import * as v8 from "v8";
v8.setFlagsFromString("--allow-natives-syntax");

import * as semver from "semver";

// On node11 and above, create an 'inspector session' that can be used to keep track of what is
// happening through a supported API.  Pre-11 we can just call into % intrinsics for the same data.
export const isNodeAtLeastV11 = semver.gte(process.version, "11.0.0");

let session: Promise<import("inspector").Session>;
if (isNodeAtLeastV11) {
    session = createInspectorSessionAsync();
}

const scriptIdToUrlMap = new Map<string, string>();

async function createInspectorSessionAsync(): Promise<import("inspector").Session> {
    // Delay loading 'inspector' as it is not available on early versions of node, so we can't
    // require it on the outside.
    const inspector = await import("inspector");
    const inspectorSession = new inspector.Session();
    inspectorSession.connect();

    // Enable debugging support so we can hear about the Debugger.scriptParsed event. We need that
    // event to know how to map from scriptId's to file-urls.
    await new Promise<import("inspector").Debugger.EnableReturnType>((resolve, reject) => {
        inspectorSession.post("Debugger.enable", (err, res) => err ? reject(err) : resolve(res));
    });

    inspectorSession.addListener("Debugger.scriptParsed", event => {
        const { scriptId, url } = event.params;
        scriptIdToUrlMap.set(scriptId, url);
    });

    return inspectorSession;
}

/**
 * Returns the inspector session that can be used to query the state of this running Node
 * instance.  Only available on Node11 and above.
 */
export function getSessionAsync() {
    if (!isNodeAtLeastV11) {
        throw new Error("Should only be calling into this module on node11 or higher.");
    }

    return session;
}

/**
 * Maps from a script-id to the local file url it corresponds to.
 */
export function getScriptUrl(id: import("inspector").Runtime.ScriptId) {
    return scriptIdToUrlMap.get(id);
}
