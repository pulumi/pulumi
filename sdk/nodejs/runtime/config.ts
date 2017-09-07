// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

let config: {[key: string]: string} = {};

// setConfig sets a configuration variable.
export function setConfig(k: string, v: string): void {
    config[k] = v;
}

// getConfig returns a configuration variable's value or undefined if it is unset.
export function getConfig(k: string): string | undefined {
    return config[k];
}

