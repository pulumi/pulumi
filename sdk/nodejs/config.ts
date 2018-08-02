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

import * as util from "util";
import { RunError } from "./errors";
import { getProject } from "./metadata";
import { getConfig } from "./runtime";

/**
 * Config is a bag of related configuration state.  Each bag contains any number of configuration variables, indexed by
 * simple keys, and each has a name that uniquely identifies it; two bags with different names do not share values for
 * variables that otherwise share the same key.  For example, a bag whose name is `pulumi:foo`, with keys `a`, `b`,
 * and `c`, is entirely separate from a bag whose name is `pulumi:bar` with the same simple key names.  Each key has a
 * fully qualified names, such as `pulumi:foo:a`, ..., and `pulumi:bar:a`, respectively.
 */
export class Config {
    /**
     * name is the configuration bag's logical name and uniquely identifies it.  The default is the name of the current
     * project.
     */
    public readonly name: string;

    constructor(name?: string) {
        if (name === undefined) {
            name = getProject();
        }

        if (name.endsWith(":config")) {
            name = name.replace(/:config$/, "");
        }

        this.name = name;
    }

    /**
     * get loads an optional configuration value by its key, or undefined if it doesn't exist.
     *
     * @param key The key to lookup.
     */
    public get(key: string): string | undefined {
        return getConfig(this.fullKey(key));
    }

    /**
     * getBoolean loads an optional configuration value, as a boolean, by its key, or undefined if it doesn't exist.
     * If the configuration value isn't a legal boolean, this function will throw an error.
     *
     * @param key The key to lookup.
     */
    public getBoolean(key: string): boolean | undefined {
        const v: string | undefined = this.get(key);
        if (v === undefined) {
            return undefined;
        } else if (v === "true") {
            return true;
        } else if (v === "false") {
            return false;
        }
        throw new ConfigTypeError(this.fullKey(key), v, "boolean");
    }

    /**
     * getNumber loads an optional configuration value, as a number, by its key, or undefined if it doesn't exist.
     * If the configuration value isn't a legal number, this function will throw an error.
     *
     * @param key The key to lookup.
     */
    public getNumber(key: string): number | undefined {
        const v: string | undefined = this.get(key);
        if (v === undefined) {
            return undefined;
        }
        const f: number = parseFloat(v);
        if (isNaN(f)) {
            throw new ConfigTypeError(this.fullKey(key), v, "number");
        }
        return f;
    }

    /**
     * getObject loads an optional configuration value, as an object, by its key, or undefined if it doesn't exist.
     * This routine simply JSON parses and doesn't validate the shape of the contents.
     *
     * @param key The key to lookup.
     */
    public getObject<T>(key: string): T | undefined {
        const v: string | undefined = this.get(key);
        if (v === undefined) {
            return undefined;
        }
        try {
            return <T>JSON.parse(v);
        }
        catch (err) {
            throw new ConfigTypeError(this.fullKey(key), v, "JSON object");
        }
    }

    /**
     * require loads a configuration value by its given key.  If it doesn't exist, an error is thrown.
     *
     * @param key The key to lookup.
     */
    public require(key: string): string {
        const v: string | undefined = this.get(key);
        if (v === undefined) {
            throw new ConfigMissingError(this.fullKey(key));
        }
        return v;
    }

    /**
     * requireBoolean loads a configuration value, as a boolean, by its given key.  If it doesn't exist, or the
     * configuration value is not a legal boolean, an error is thrown.
     *
     * @param key The key to lookup.
     */
    public requireBoolean(key: string): boolean {
        const v: boolean | undefined = this.getBoolean(key);
        if (v === undefined) {
            throw new ConfigMissingError(this.fullKey(key));
        }
        return v;
    }

    /**
     * requireNumber loads a configuration value, as a number, by its given key.  If it doesn't exist, or the
     * configuration value is not a legal number, an error is thrown.
     *
     * @param key The key to lookup.
     */
    public requireNumber(key: string): number {
        const v: number | undefined = this.getNumber(key);
        if (v === undefined) {
            throw new ConfigMissingError(this.fullKey(key));
        }
        return v;
    }

    /**
     * requireObject loads a configuration value, as a number, by its given key.  If it doesn't exist, or the
     * configuration value is not a legal number, an error is thrown.
     *
     * @param key The key to lookup.
     */
    public requireObject<T>(key: string): T {
        const v: T | undefined = this.getObject<T>(key);
        if (v === undefined) {
            throw new ConfigMissingError(this.fullKey(key));
        }
        return v;
    }

    /**
     * fullKey turns a simple configuration key into a fully resolved one, by prepending the bag's name.
     *
     * @param key The key to lookup.
     */
    private fullKey(key: string): string {
        return `${this.name}:${key}`;
    }
}

/**
 * ConfigTypeError is used when a configuration value is of the wrong type.
 */
class ConfigTypeError extends RunError {
    constructor(key: string, v: any, expectedType: string) {
        super(`Configuration '${key}' value '${v}' is not a valid ${expectedType}`);
    }
}

/**
 * ConfigMissingError is used when a configuration value is completely missing.
 */
class ConfigMissingError extends RunError {
    constructor(public key: string) {
        super(
            `Missing required configuration variable '${key}'\n` +
            `\tplease set a value using the command \`pulumi config set ${key} <value>\``,
        );
    }
}
