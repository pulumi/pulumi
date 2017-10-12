// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

/**
 * configEnvKey is the environment variable key for configuration that we will check in the event that a
 * configuration variable is missing.  Explicit overrides take precedence.
 */
export const configEnvKey = "PULUMI_CONFIG";

let config: {[key: string]: string} = {};

/**
 * setConfig sets a configuration variable.
 */
export function setConfig(k: string, v: string): void {
    config[k] = v;
}

/**
 * getConfig returns a configuration variable's value or undefined if it is unset.
 */
export function getConfig(k: string): string | undefined {
    // If the config has been set explicitly, use it.
    if (config.hasOwnProperty(k)) {
        return config[k];
    }

    // If there is a specific PULUMI_CONFIG_<k> variable, use it.
    const envKey: string = getConfigEnvKey(k);
    if (process.env.hasOwnProperty(envKey)) {
        return process.env[envKey];
    }

    // If the config hasn't been set, but there is a process-wide PULUMI_CONFIG envvar, use it.
    const envObject: {[key: string]: string} = getConfigEnv();
    if (envObject.hasOwnProperty(k)) {
        return envObject[k];
    }

    return undefined;
}

/**
 * getConfigEnvKey returns a scrubbed environment variable key, PULUMI_CONFIG_<k>, that can be used for
 * setting explicit varaibles.  This is unlike PULUMI_CONFIG which is just a JSON-serialized bag.
 */
export function getConfigEnvKey(key: string): string {
    let envkey: string = "";
    for (let c of key) {
        if (c == '_' || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
            envkey += c;
        }
        else if (c >= 'a' && c <= 'z') {
            envkey += c.toUpperCase();
        }
        else {
            envkey += '_';
        }
    }
    return `${configEnvKey}_${envkey}`;
}

/**
 * getConfigEnv returns the environment map that will be used for config checking when variables aren't set.
 */
export function getConfigEnv(): {[key: string]: string} {
    const envConfig = process.env.PULUMI_CONFIG;
    if (envConfig) {
        return JSON.parse(envConfig);
    }
    return {};
}

