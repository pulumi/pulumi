// Copyright 2016-2019, Pulumi Corporation.
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

/**
 * stackTagsEnvKey is the environment variable key that the language plugin uses to set stack tags.
 */
const stackTagsEnvKey = "PULUMI_STACK_TAGS";

const tags: {[name: string]: string} = parseStackTags();

/**
 * allStackTags returns a copy of the full tags map.
 */
export function allStackTags(): {[name: string]: string} {
    return Object.assign({}, tags);
}

/**
 * getStackTag returns a stack tag's value or undefined if it is unset.
 */
export function getStackTag(name: string): string | undefined {
    return tags[name];
}

function parseStackTags() {
    const parsedStackTags: {[name: string]: string} = {};
    const envStackTags = process.env[stackTagsEnvKey];
    if (envStackTags) {
        const envObject: {[name: string]: string} = JSON.parse(envStackTags);
        for (const k of Object.keys(envObject)) {
            parsedStackTags[k] = envObject[k];
        }
    }

    return parsedStackTags;
}
