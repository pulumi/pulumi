// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as runtime from "./runtime";

// Config is a bag of related configuration state.  Each bag contains any number of configuration variables, indexed by
// simple keys, and each has a name that uniquely identifies it; two bags with different names do not share values for
// variables that otherwise share the same key.  For example, a bag whose name is `pulumi:foo`, with keys `a`, `b`,
// and `c`, is entirely separate from a bag whose name is `pulumi:bar` with the same simple key names.  Each key has a
// fully qualified names, such as `pulumi:foo:a`, ..., and `pulumi:bar:a`, respectively.
export class Config {
    // name is the configuration bag's logical name and uniquely identifies it.
    public readonly name: string;

    constructor(name: string) {
        this.name = name;
    }

    // get loads an optional configuration value by its key, or undefined if it doesn't exist.
    public get(key: string): string | undefined {
        return runtime.getConfig(this.fullKey(key));
    }

    // require loads a configuration value by its given key.  If it doesn't exist, an error is thrown.
    public require(key: string): string {
        let v: string | undefined = this.get(key);
        if (v === undefined) {
            throw new Error(`Missing required configuration variable '${this.fullKey(key)}'`);
        }
        return v;
    }

    // fullKey turns a simple configuration key into a fully resolved one, by prepending the bag's name.
    private fullKey(key: string): string {
        return `${this.name}:${key}`;
    }
}

