// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

/**
 * configEnvKey is the environment variable key that the language plugin uses to set configuration values.
 */
const configEnvKey = "PULUMI_CONFIG";

const config: {[key: string]: string} = {};

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
    // If the config has been set explicitly, use it.
    if (config.hasOwnProperty(k)) {
        return config[k];
    }

    return undefined;
}

/**
 * loadConfig populates the runtime.config object based on configuration set in the environment.
 */
export function loadConfig() {
    const envConfig = process.env.PULUMI_CONFIG;
    if (envConfig) {
        const envObject: {[key: string]: string} = JSON.parse(envConfig);
        for (const key of Object.keys(envObject)) {
            setConfig(key, envObject[key]);
        }
    }
}

/**
 * cleanKey takes a configuration key, and if it is of the form "<string>:config:<string>" removes the ":config:"
 * portion. Previously, our keys always had the string ":config:" in them, and we'd like to remove it. However, the
 * language host needs to continue to set it so we can be compatable with older versions of our packages. Once we
 * stop supporting older packages, we can change the language host to not add this :config: thing and remove this
 * function.
 */
function cleanKey(key: string): string {
    const idx = key.indexOf(":");

    if (idx > 0 && key.startsWith("config:", idx + 1)) {
        return key.substring(0, idx) + ":" + key.substring(idx + 1 + "config:".length);
    }

    return key;
}
