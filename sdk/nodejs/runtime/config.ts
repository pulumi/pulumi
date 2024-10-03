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

import { getStore } from "./state";

/**
 * The environment variable key that the language plugin uses to set
 * configuration values.
 *
 * @internal
 */
export const configEnvKey = "PULUMI_CONFIG";

/**
 * The environment variable key that the language plugin uses to set
 * configuration keys that contain secrets.
 *
 * @internal
 */
export const configSecretKeysEnvKey = "PULUMI_CONFIG_SECRET_KEYS";

/**
 * Returns a copy of the full configuration map.
 */
export function allConfig(): { [key: string]: string } {
    const config = parseConfig();
    return Object.assign({}, config);
}

/**
 * Overwrites the configuration map.
 */
export function setAllConfig(c: { [key: string]: string }, secretKeys?: string[]) {
    const obj: { [key: string]: string } = {};
    for (const k of Object.keys(c)) {
        obj[cleanKey(k)] = c[k];
    }
    persistConfig(obj, secretKeys);
}

/**
 * Sets a configuration variable.
 */
export function setConfig(k: string, v: string): void {
    const config = parseConfig();
    config[cleanKey(k)] = v;
    persistConfig(config, []);
}

/**
 * Returns a configuration variable's value, or `undefined` if it is unset.
 */
export function getConfig(k: string): string | undefined {
    const config = parseConfig();
    return config[k];
}

/**
 * Returns true if the key contains a secret value.
 *
 * @internal
 */
export function isConfigSecret(k: string): boolean {
    const { config } = getStore();
    const envConfigSecretKeys = config[configSecretKeysEnvKey];
    if (envConfigSecretKeys) {
        const envConfigSecretArray = JSON.parse(envConfigSecretKeys);
        if (Array.isArray(envConfigSecretArray)) {
            return envConfigSecretArray.includes(k);
        }
    }
    return false;
}

/**
 * Reads configuration from the source of truth, the environment. Configuration
 * must always be read this way because the Automation API introduces new
 * program lifetime semantics where program lifetime != module lifetime.
 */
function parseConfig() {
    const { config } = getStore();
    const parsedConfig: { [key: string]: string } = {};
    const envConfig = config[configEnvKey];
    if (envConfig) {
        const envObject: { [key: string]: string } = JSON.parse(envConfig);
        for (const k of Object.keys(envObject)) {
            parsedConfig[cleanKey(k)] = envObject[k];
        }
    }

    return parsedConfig;
}

/**
 * Writes configuration to the environment. Configuration changes must always be
 * persisted to the environment this way because the Automation API introduces
 * new program lifetime semantics where program lifetime != module lifetime.
 */
function persistConfig(config: { [key: string]: string }, secretKeys?: string[]) {
    const store = getStore();
    const serializedConfig = JSON.stringify(config);
    const serializedSecretKeys = Array.isArray(secretKeys) ? JSON.stringify(secretKeys) : "[]";
    store.config[configEnvKey] = serializedConfig;
    store.config[configSecretKeysEnvKey] = serializedSecretKeys;
}

/**
 * Takes a configuration key and, if it is of the form
 * "<string>:config:<string>" removes the ":config:" portion. Previously, our
 * keys always had the string ":config:" in them, and we'd like to remove it.
 * However, the language host needs to continue to set it so we can be
 * compatible with older versions of our packages. Once we stop supporting older
 * packages, we can change the language host to not add this :config: thing and
 * remove this function.
 */
function cleanKey(key: string): string {
    const idx = key.indexOf(":");

    if (idx > 0 && key.startsWith("config:", idx + 1)) {
        return key.substring(0, idx) + ":" + key.substring(idx + 1 + "config:".length);
    }

    return key;
}
