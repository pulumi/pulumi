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

import { RunError } from "./errors";
import * as log from "./log";
import { getProject } from "./metadata";
import { Output } from "./output";
import { getConfig, isConfigSecret } from "./runtime";

function makeSecret<T>(value: T): Output<T> {
    return new Output(
        [], Promise.resolve(value),
        /*isKnown:*/ Promise.resolve(true), /*isSecret:*/ Promise.resolve(true),
        Promise.resolve([]));
}

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
     * @param opts An options bag to constrain legal values.
     */
    private getImpl<K extends string = string>(key: string,
                                               opts?: StringConfigOptions<K>,
                                               use?: (...args: any[]) => any,
                                               insteadOf?: (...args: any[]) => any): K | undefined {
        const fullKey = this.fullKey(key);
        const v: string | undefined = getConfig(fullKey);
        if (v === undefined) {
            return undefined;
        }
        // TODO[pulumi/pulumi#7127]: Re-enable the warning.
        // Temporarily disabling the new warning.
        // if (use && insteadOf && isConfigSecret(fullKey)) {
        //     log.warn(`Configuration '${fullKey}' value is a secret; ` +
        //         `use \`${use.name}\` instead of \`${insteadOf.name}\``);
        // }
        if (opts) {
            // SAFETY: if allowedValues != null, verifying v ∈ K[]
            if (opts.allowedValues !== undefined && opts.allowedValues.indexOf(v as any) === -1) {
                throw new ConfigEnumError(fullKey, v, opts.allowedValues);
            } else if (opts.minLength !== undefined && v.length < opts.minLength) {
                throw new ConfigRangeError(fullKey, v, opts.minLength, undefined);
            } else if (opts.maxLength !== undefined && v.length > opts.maxLength) {
                throw new ConfigRangeError(fullKey, v, undefined, opts.maxLength);
            } else if (opts.pattern !== undefined) {
                let pattern = opts.pattern;
                if (typeof pattern === "string") {
                    pattern = new RegExp(pattern);
                }
                if (!pattern.test(v)) {
                    throw new ConfigPatternError(fullKey, v, pattern);
                }
            }
        }
        // SAFETY:
        // allowedValues != null ⇒ v ∈ K[]
        // allowedValues == null ⇒ K = string & v : string
        return v as K;
    }

    /**
     * get loads an optional configuration value by its key, or undefined if it doesn't exist.
     *
     * @param key The key to lookup.
     * @param opts An options bag to constrain legal values.
     */
    public get<K extends string = string>(key: string, opts?: StringConfigOptions<K>): K | undefined {
        return this.getImpl(key, opts, this.getSecret, this.get);
    }

    /**
     * getSecret loads an optional configuration value by its key, marking it as a secret, or undefined if it
     * doesn't exist.
     *
     * @param key The key to lookup.
     * @param opts An options bag to constrain legal values.
     */
    public getSecret<K extends string = string>(key: string, opts?: StringConfigOptions<K>): Output<K> | undefined {
        const v = this.getImpl(key, opts);
        if (v === undefined) {
            return undefined;
        }

        return makeSecret(v);
    }

    private getBooleanImpl(key: string,
                           use?: (...args: any[]) => any,
                           insteadOf?: (...args: any[]) => any): boolean | undefined {
        const v: string | undefined = this.getImpl(key, undefined, use, insteadOf);
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
     * getBoolean loads an optional configuration value, as a boolean, by its key, or undefined if it doesn't exist.
     * If the configuration value isn't a legal boolean, this function will throw an error.
     *
     * @param key The key to lookup.
     */
    public getBoolean(key: string): boolean | undefined {
        return this.getBooleanImpl(key, this.getSecretBoolean, this.getBoolean);
    }

    /**
     * getSecretBoolean loads an optional configuration value, as a boolean, by its key, making it as a secret
     * or undefined if it doesn't exist. If the configuration value isn't a legal boolean, this function will
     * throw an error.
     *
     * @param key The key to lookup.
     */
    public getSecretBoolean(key: string): Output<boolean> | undefined {
        const v = this.getBooleanImpl(key);
        if (v === undefined) {
            return undefined;
        }

        return makeSecret(v);
    }

    private getNumberImpl(key: string,
                          opts?: NumberConfigOptions,
                          use?: (...args: any[]) => any,
                          insteadOf?: (...args: any[]) => any): number | undefined {
        const v: string | undefined = this.getImpl(key, undefined, use, insteadOf);
        if (v === undefined) {
            return undefined;
        }
        const f: number = parseFloat(v);
        if (isNaN(f)) {
            throw new ConfigTypeError(this.fullKey(key), v, "number");
        }
        if (opts) {
            if (opts.min !== undefined && f < opts.min) {
                throw new ConfigRangeError(this.fullKey(key), f, opts.min, undefined);
            } else if (opts.max !== undefined && f > opts.max) {
                throw new ConfigRangeError(this.fullKey(key), f, undefined, opts.max);
            }
        }
        return f;
    }

    /**
     * getNumber loads an optional configuration value, as a number, by its key, or undefined if it doesn't exist.
     * If the configuration value isn't a legal number, this function will throw an error.
     *
     * @param key The key to lookup.
     * @param opts An options bag to constrain legal values.
     */
    public getNumber(key: string, opts?: NumberConfigOptions): number | undefined {
        return this.getNumberImpl(key, opts, this.getSecretNumber, this.getNumber);
    }

    /**
     * getSecretNumber loads an optional configuration value, as a number, by its key, marking it as a secret
     * or undefined if it doesn't exist.
     * If the configuration value isn't a legal number, this function will throw an error.
     *
     * @param key The key to lookup.
     * @param opts An options bag to constrain legal values.
     */
    public getSecretNumber(key: string, opts?: NumberConfigOptions): Output<number> | undefined {
        const v = this.getNumberImpl(key, opts);
        if (v === undefined) {
            return undefined;
        }

        return makeSecret(v);
    }

    private getObjectImpl<T>(key: string,
                             use?: (...args: any[]) => any,
                             insteadOf?: (...args: any[]) => any): T | undefined {
        const v: string | undefined = this.getImpl(key, undefined, use, insteadOf);
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
     * getObject loads an optional configuration value, as an object, by its key, or undefined if it doesn't exist.
     * This routine simply JSON parses and doesn't validate the shape of the contents.
     *
     * @param key The key to lookup.
     */
    public getObject<T>(key: string): T | undefined {
        return this.getObjectImpl<T>(key, this.getSecretObject, this.getObject);
    }

    /**
     * getSecretObject loads an optional configuration value, as an object, by its key, marking it as a secret
     * or undefined if it doesn't exist.
     * This routine simply JSON parses and doesn't validate the shape of the contents.
     *
     * @param key The key to lookup.
     */
    public getSecretObject<T>(key: string): Output<T> | undefined {
        const v = this.getObjectImpl<T>(key);

        if (v === undefined) {
            return undefined;
        }

        return makeSecret<T>(v);
    }

    private requireImpl<K extends string = string>(key: string,
                                                   opts?: StringConfigOptions<K>,
                                                   use?: (...args: any[]) => any,
                                                   insteadOf?: (...args: any[]) => any): K {
        const v: K | undefined = this.getImpl(key, opts, use, insteadOf);
        if (v === undefined) {
            throw new ConfigMissingError(this.fullKey(key));
        }
        return v;
    }

    /**
     * require loads a configuration value by its given key.  If it doesn't exist, an error is thrown.
     *
     * @param key The key to lookup.
     * @param opts An options bag to constrain legal values.
     */
    public require<K extends string = string>(key: string, opts?: StringConfigOptions<K>): K {
        return this.requireImpl(key, opts, this.requireSecret, this.require);
    }

    /**
     * require loads a configuration value by its given key, marking it as a secret.  If it doesn't exist, an error
     * is thrown.
     *
     * @param key The key to lookup.
     * @param opts An options bag to constrain legal values.
     */
    public requireSecret<K extends string = string>(key: string, opts?: StringConfigOptions<K>): Output<K> {
        return makeSecret(this.requireImpl(key, opts));
    }

    private requireBooleanImpl(key: string,
                               use?: (...args: any[]) => any,
                               insteadOf?: (...args: any[]) => any): boolean {
        const v: boolean | undefined = this.getBooleanImpl(key, use, insteadOf);
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
        return this.requireBooleanImpl(key, this.requireSecretBoolean, this.requireBoolean);
    }

    /**
     * requireSecretBoolean loads a configuration value, as a boolean, by its given key, marking it as a secret.
     * If it doesn't exist, or the configuration value is not a legal boolean, an error is thrown.
     *
     * @param key The key to lookup.
     */
    public requireSecretBoolean(key: string): Output<boolean> {
        return makeSecret(this.requireBooleanImpl(key));
    }

    private requireNumberImpl(key: string,
                              opts?: NumberConfigOptions,
                              use?: (...args: any[]) => any,
                              insteadOf?: (...args: any[]) => any): number {
        const v: number | undefined = this.getNumberImpl(key, opts, use, insteadOf);
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
     * @param opts An options bag to constrain legal values.
     */
    public requireNumber(key: string, opts?: NumberConfigOptions): number {
        return this.requireNumberImpl(key, opts, this.requireSecretNumber, this.requireNumber);
    }

    /**
     * requireSecretNumber loads a configuration value, as a number, by its given key, marking it as a secret.
     * If it doesn't exist, or the configuration value is not a legal number, an error is thrown.
     *
     * @param key The key to lookup.
     * @param opts An options bag to constrain legal values.
     */
    public requireSecretNumber(key: string, opts?: NumberConfigOptions): Output<number> {
        return makeSecret(this.requireNumberImpl(key, opts));
    }

    private requireObjectImpl<T>(key: string, use?: (...args: any[]) => any, insteadOf?: (...args: any[]) => any): T {
        const v: T | undefined = this.getObjectImpl<T>(key, use, insteadOf);
        if (v === undefined) {
            throw new ConfigMissingError(this.fullKey(key));
        }
        return v;
    }

    /**
     * requireObject loads a configuration value as a JSON string and deserializes the JSON into a JavaScript object. If
     * it doesn't exist, or the configuration value is not a legal JSON string, an error is thrown.
     *
     * @param key The key to lookup.
     */
    public requireObject<T>(key: string): T {
        return this.requireObjectImpl<T>(key, this.requireSecretObject, this.requireObject);
    }

    /**
     * requireSecretObject loads a configuration value as a JSON string and deserializes the JSON into a JavaScript
     * object, marking it as a secret. If it doesn't exist, or the configuration value is not a legal JSON
     * string, an error is thrown.
     *
     * @param key The key to lookup.
     */
    public requireSecretObject<T>(key: string): Output<T> {
        return makeSecret(this.requireObjectImpl<T>(key));
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
 * StringConfigOptions may be used to constrain the set of legal values a string config value may contain.
 */
export interface StringConfigOptions<K extends string = string> {
    /**
     * The legal enum values. If it does not match, a ConfigEnumError is thrown.
     */
    allowedValues?: K[];
    /**
     * The minimum string length. If the string is not this long, a ConfigRangeError is thrown.
     */
    minLength?: number;
    /**
     * The maximum string length. If the string is longer than this, a ConfigRangeError is thrown.
     */
    maxLength?: number;
    /**
     * A regular expression the string must match. If it does not match, a ConfigPatternError is thrown.
     */
    pattern?: string | RegExp;
}

/**
 * NumberConfigOptions may be used to constrain the set of legal values a number config value may contain.
 */
export interface NumberConfigOptions {
    /**
     * The minimum number value, inclusive. If the number is less than this, a ConfigRangeError is thrown.
     */
    min?: number;
    /**
     * The maximum number value, inclusive. If the number is greater than this, a ConfigRangeError is thrown.
     */
    max?: number;
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
