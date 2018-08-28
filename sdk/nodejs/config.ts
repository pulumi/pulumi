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
     * getEnum loads an optional configuration value by its key, or undefined if it doesn't exist. If the value is not
     * within the array of legal values, an error will be thrown.
     *
     * @param key The key to lookup.
     * @param values The legal enum values.
     */
    public getEnum(key: string, values: string[]): string | undefined {
        const v = getConfig(this.fullKey(key));
        if (v !== undefined && values.indexOf(v) === -1) {
            throw new ConfigEnumError(this.fullKey(key), v, values);
        }
        return v;
    }

    /**
     * getMinMax loads an optional string configuration value by its key, or undefined if it doesn't exist. If the
     * value's length is less than or greater than the specified number of characters, this function throws.
     *
     * @param key The key to lookup.
     * @param min The minimum string length.
     * @param max The maximum string length.
     */
    public getMinMax(key: string, min: number, max: number): string | undefined {
        const v = getConfig(this.fullKey(key));
        if (v !== undefined && (v.length < min || v.length > max)) {
            throw new ConfigRangeError(this.fullKey(key), v, min, max);
        }
        return v;
    }

    /**
     * getMinMaxPattern loads an optional string configuration value by its key, or undefined if it doesn't exist. If
     * the value's length is less than or greater than the specified number of characters, or the string does not match
     * the supplied regular expression, this function throws.
     *
     * @param key The key to lookup.
     * @param min The minimum string length.
     * @param max The maximum string length.
     * @param regexp A regular expression the string must match.
     */
    public getMinMaxPattern(key: string, min: number, max: number, regexp: string | RegExp): string | undefined {
        if (typeof regexp === "string") {
            regexp = new RegExp(regexp);
        }

        const v = getConfig(this.fullKey(key));
        if (v !== undefined) {
            if (v.length < min || v.length > max) {
                throw new ConfigRangeError(this.fullKey(key), v, min, max);
            } else if (!regexp.test(v)) {
                throw new ConfigPatternError(this.fullKey(key), v, regexp);
            }
        }
        return v;
    }

    /**
     * getPattern loads an optional string configuration value by its key, or undefined if it doesn't exist. If the
     * value doesn't match the regular expression pattern, the function throws.
     *
     * @param key The key to lookup.
     * @param regexp A regular expression the string must match.
     */
    public getPattern(key: string, regexp: string | RegExp): string | undefined {
        if (typeof regexp === "string") {
            regexp = new RegExp(regexp);
        }

        const v = getConfig(this.fullKey(key));
        if (v !== undefined && !regexp.test(v)) {
            throw new ConfigPatternError(this.fullKey(key), v, regexp);
        }
        return v;
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
     * getNumberMinMax loads an optional configuration value, as a number, by its key, or undefined if it doesn't exist.
     * If the configuration value isn't a legal number, this function will throw an error. The range is a pair of min
     * and max values that, should there be a value, the number must fall within, inclusively, else an error is thrown.
     *
     * @param key The key to lookup.
     * @param min The minimum value the number may be, inclusive.
     * @param max The maximum value the number may be, inclusive.
     */
    public getNumberMinMax(key: string, min: number, max: number): number | undefined {
        const v: number | undefined = this.getNumber(key);
        if (v !== undefined && (v < min || v > max)) {
            throw new ConfigRangeError(this.fullKey(key), v, min, max);
        }
        return v;
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
     * requireEnum loads a configuration value by its given key.  If it doesn't exist, an error is thrown. If the value
     * is not within the array of legal values, an error will be thrown.
     *
     * @param key The key to lookup.
     * @param values The legal enum values.
     */
    public requireEnum(key: string, values: string[]): string {
        const v: string | undefined = this.getEnum(key, values);
        if (v === undefined) {
            throw new ConfigMissingError(this.fullKey(key));
        }
        return v;
    }

    /**
     * requireMinMax loads a string configuration value by its key. If it doesn't exist, an error is thrown. If the
     * value's length is less than or greater than the specified number of characters, this function throws.
     *
     * @param key The key to lookup.
     * @param min The minimum string length.
     * @param max The maximum string length.
     */
    public requireMinMax(key: string, min: number, max: number): string {
        const v = this.getMinMax(key, min, max);
        if (v === undefined) {
            throw new ConfigMissingError(this.fullKey(key));
        }
        return v;
    }

    /**
     * requireMinMaxPattern loads a string configuration value by its key. If it doesn't exist, an error is thrown. If
     * the value's length is less than or greater than the specified number of characters, this function throws.
     *
     * @param key The key to lookup.
     * @param min The minimum string length.
     * @param max The maximum string length.
     * @param regexp A regular expression the string must match.
     */
    public requireMinMaxPattern(key: string, min: number, max: number, pattern: string | RegExp): string {
        const v = this.getMinMaxPattern(key, min, max, pattern);
        if (v === undefined) {
            throw new ConfigMissingError(this.fullKey(key));
        }
        return v;
    }

    /**
     * requirePattern loads a string configuration value by its key. If it doesn't exist, an error is thrown. If the
     * value's length is less than or greater than the specified number of characters, this function throws.
     *
     * @param key The key to lookup.
     * @param regexp A regular expression the string must match.
     */
    public requirePattern(key: string, pattern: string | RegExp): string {
        const v = this.getPattern(key, pattern);
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
     * requireNumberMinMax loads a configuration value, as a number, by its given key.  If it doesn't exist, or the
     * configuration value is not a legal number, an error is thrown. The range is a pair of min and max values that,
     * should there be a value, the number must fall within, inclusively, else an error is thrown.
     *
     * @param key The key to lookup.
     * @param min The minimum value the number may be, inclusive.
     * @param max The maximum value the number may be, inclusive.
     */
    public requireNumberMinMax(key: string, min: number, max: number): number {
        const v: number | undefined = this.getNumberMinMax(key, min, max);
        if (v === undefined) {
            throw new ConfigMissingError(this.fullKey(key));
        }
        return v;
    }

    /**
     * requireNumberMinMax loads a configuration value, as a number, by its given key.  If it doesn't exist, or the
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
 * ConfigEnumError is used when a configuration value isn't a correct enum value.
 */
class ConfigEnumError extends RunError {
    constructor(key: string, v: any, values: any[]) {
        super(`Configuration '${key}' value '${v}' is not a legal enum value (${JSON.stringify(values)})`);
   }
}

/**
 * ConfigRangeError is used when a configuration value is outside of the range of legal sizes.
 */
class ConfigRangeError extends RunError {
    constructor(key: string, v: any, min: number | undefined, max: number | undefined) {
        let range: string;
        if (max === undefined) {
            range = `min ${min}`;
        } else if (min === undefined) {
            range = `max ${max}`;
        } else {
            range = `${min}-${max}`;
        }
        if (typeof v === "string") {
            range += " chars";
        }
        super(`Configuration '${key}' value '${v}' is outside of the legal range (${range}, inclusive)`);
   }
}

/**
 * ConfigPatternError is used when a configuration value does not match the given regular expression.
 */
class ConfigPatternError extends RunError {
    constructor(key: string, v: string, regexp: RegExp) {
        super(`Configuration '${key}' value '${v}' does not match the regular expression ${regexp.toString()}`);
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
