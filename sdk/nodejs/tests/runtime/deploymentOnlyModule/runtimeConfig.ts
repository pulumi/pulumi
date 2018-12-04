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

// Copy of real 'runtime/config.ts' file to ensure that if we capture somehting marked as
// deployment-time that any code it captures (even through modules), should be captured by value.

/**
 * configEnvKey is the environment variable key that the language plugin uses to set configuration values.
 */

const config: {[key: string]: string} = {};

/**
 * allConfig returns a copy of the full config map.
 */
export function allConfig(): {[key: string]: string} {
    return Object.assign({}, config);
}

/**
 * setConfig sets a configuration variable.
 */
export function setConfig(k: string, v: string): void {
    config[cleanKey(k)] = v;
}

/**
 * getConfig returns a configuration variable's value or undefined if it is unset.
 */
export function getConfig(k: string): string | undefined {
    return config[k];
}

/**
 * cleanKey takes a configuration key, and if it is of the form "<string>:config:<string>" removes
 * the ":config:" portion. Previously, our keys always had the string ":config:" in them, and we'd
 * like to remove it. However, the language host needs to continue to set it so we can be compatible
 * with older versions of our packages. Once we stop supporting older packages, we can change the
 * language host to not add this :config: thing and remove this function.
 */
function cleanKey(key: string): string {
    const idx = key.indexOf(":");

    if (idx > 0 && key.startsWith("config:", idx + 1)) {
        return key.substring(0, idx) + ":" + key.substring(idx + 1 + "config:".length);
    }

    return key;
}
