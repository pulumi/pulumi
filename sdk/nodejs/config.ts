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

    // getBoolean loads an optional configuration value, as a boolean, by its key, or undefined if it doesn't exist.
    // If the configuration value isn't a legal boolean, this function will throw an error.
    public getBoolean(key: string): boolean | undefined {
        let v: string | undefined = this.get(key);
        if (v === undefined) {
            return undefined;
        } else if (v === "true") {
            return true;
        } else if (v === "false") {
            return false;
        }
        throw new Error(`Configuration '${key}' value '${v}' is not a valid boolean`);
    }

    // getNumber loads an optional configuration value, as a number, by its key, or undefined if it doesn't exist.
    // If the configuration value isn't a legal number, this function will throw an error.
    public getNumber(key: string): number | undefined {
        let v: string | undefined = this.get(key);
        if (v === undefined) {
            return undefined;
        }
        let f: number = parseFloat(v);
        if (isNaN(f)) {
            throw new Error(`Configuration '${key}' value '${v}' is not a valid number`);
        }
        return f;
    }

    // getObject loads an optional configuration value, as an object, by its key, or undefined if it doesn't exist.
    // This routine simply JSON parses and doesn't validate the shape of the contents.
    public getObject<T>(key: string): T | undefined {
        let v: string | undefined = this.get(key);
        if (v === undefined) {
            return undefined;
        }
        return <T>JSON.parse(v);
    }

    // require loads a configuration value by its given key.  If it doesn't exist, an error is thrown.
    public require(key: string): string {
        let v: string | undefined = this.get(key);
        if (v === undefined) {
            throw new Error(`Missing required configuration variable '${this.fullKey(key)}'`);
        }
        return v;
    }

    // requireBoolean loads a configuration value, as a boolean, by its given key.  If it doesn't exist, or the
    // configuration value is not a legal boolean, an error is thrown.
    public requireBoolean(key: string): boolean {
        let v: boolean | undefined = this.getBoolean(key);
        if (v === undefined) {
            throw new Error(`Missing required configuration variable '${this.fullKey(key)}'`);
        }
        return v;
    }

    // requireNumber loads a configuration value, as a number, by its given key.  If it doesn't exist, or the
    // configuration value is not a legal number, an error is thrown.
    public requireNumber(key: string): number {
        let v: number | undefined = this.getNumber(key);
        if (v === undefined) {
            throw new Error(`Missing required configuration variable '${this.fullKey(key)}'`);
        }
        return v;
    }

    // requireObject loads a configuration value, as a number, by its given key.  If it doesn't exist, or the
    // configuration value is not a legal number, an error is thrown.
    public requireObject<T>(key: string): T {
        let v: T | undefined = this.getObject<T>(key);
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

