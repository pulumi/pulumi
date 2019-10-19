// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using Newtonsoft.Json.Linq;

namespace Pulumi
{
    /// <summary>
    /// Config is a bag of related configuration state.  Each bag contains any number of
    /// configuration variables, indexed by simple keys, and each has a name that uniquely
    /// identifies it; two bags with different names do not share values for variables that
    /// otherwise share the same key.  For example, a bag whose name is <c>pulumi:foo</c>, with keys <c>a</c>,
    /// <c>b</c>, and <c>c</c>, is entirely separate from a bag whose name is <c>pulumi:bar</c> with
    /// the same simple key names.  Each key has a fully qualified names, such as
    /// <c>pulumi:foo:a</c>, ..., and <c>pulumi:bar:a</c>, respectively.
    /// </summary>
    public class Config
    {
        /// <summary>
        /// name is the configuration bag's logical name and uniquely identifies it.  The default
        /// is the name of the current project.
        /// </summary>
        public readonly string Name;

        public Config(string? name = null)
        {
            if (name == null)
            {
                name = Deployment.Instance.Options.Project;
            }

            if (name.EndsWith(":config"))
            {
                name = name[0..^":config".Length];
            }

            this.Name = name;
        }

        private static Output<T> MakeSecret<T>(T value) {
            return new Output<T>(
                ImmutableHashSet<Resource>.Empty,
                Task.FromResult(new OutputData<T>(value, isKnown: true, isSecret: true)));
        }

        /// <summary>
        /// Loads an optional configuration value by its key, or <see langword="null"/> if it doesn't exist.
        /// </summary>
        public string? Get(string key, StringConfigOptions? options = null)
        {
            var v = GetConfig(this.FullKey(key));
            if (v == null) {
                return null;
            }
            if (options != null) {
                // SAFETY: if allowedValues != null, verifying v ∈ K[]
                if (options.AllowedValues !== null && opts.allowedValues.indexOf(v as any) == -1) {
                    throw new ConfigEnumError(this.FullKey(key), v, opts.allowedValues);
                } else if (opts.minLength !== null && v.length < opts.minLength) {
                    throw new ConfigRangeError(this.FullKey(key), v, opts.minLength, null);
                } else if (opts.maxLength !== null && v.length > opts.maxLength) {
                    throw new ConfigRangeError(this.FullKey(key), v, null, opts.maxLength);
                } else if (opts.pattern !== null) {
                    var pattern = opts.pattern;
                    if (typeof pattern == "string") {
                        pattern = new RegExp(pattern);
                    }
                    if (!pattern.test(v)) {
                        throw new ConfigPatternError(this.FullKey(key), v, pattern);
                    }
                }
            }

            return v;
        }

        /// <summary>
        /// Loads an optional configuration value by its key, marking it as a secret, or <see
        /// langword="null"/> if it doesn't exist.
        /// </summary>
        public Output<string>? GetSecret(string key, StringConfigOptions? opts = null) {
            var v = this.Get(key, opts);
            if (v == null) {
                return null;
            }

            return MakeSecret(v);
        }

        /// <summary>
        /// Loads an optional configuration value, as a boolean, by its key, or null if it doesn't exist.
        /// If the configuration value isn't a legal boolean, this function will throw an error.
        /// </summary>
        public bool? GetBool(string key) {
            var v = this.Get(key);
            if (v == null) {
                return null;
            } else if (v == "true") {
                return true;
            } else if (v == "false") {
                return false;
            }
            throw new ConfigTypeError(this.FullKey(key), v, "bool");
        }

        /// <summary>
        /// loads an optional configuration value, as a boolean, by its key, making it as a secret or
        /// null if it doesn't exist. If the configuration value isn't a legal boolean, this
        /// function will throw an error.
        /// </summary>
        public Output<bool>? GetSecretBool(string key)
        {
            var v = this.GetBool(key);
            if (v == null) {
                return null;
            }

            return MakeSecret(v);
        }

        /*
         * getInt32 loads an optional configuration value, as a number, by its key, or null if it doesn't exist.
         * If the configuration value isn't a legal number, this function will throw an error.
         *
         * @param key The key to lookup.
         * @param opts An options bag to constrain legal values.
         */
        /// <summary>
        /// Loads an optional configuration value, as a number, by its key, or null if it doesn't exist.
        /// If the configuration value isn't a legal number, this function will throw an error.
        /// </summary>
        public int? GetInt32(string key, Int32ConfigOptions? opts = null)
        {
            var v = this.Get(key);
            if (v == null) {
                return null;
            }

            if (!int.TryParse(v, out var result))
            {
                throw new ConfigTypeError(this.FullKey(key), v, "Int32");
            }

            if (opts != null) {
                if (opts.min !== null && f < opts.min)
                {
                    throw new ConfigRangeError(this.FullKey(key), f, opts.min, null);
                }
                else if (opts.max !== null && f > opts.max)
                {
                    throw new ConfigRangeError(this.FullKey(key), f, null, opts.max);
                }
            }
            return result;
        }

        /*
         * getSecretInt32 loads an optional configuration value, as a number, by its key, marking it as a secret
         * or null if it doesn't exist.
         * If the configuration value isn't a legal number, this function will throw an error.
         *
         * @param key The key to lookup.
         * @param opts An options bag to constrain legal values.
         */
        /// <summary>
        /// loads an optional configuration value, as a number, by its key, marking it as a secret
        /// or null if it doesn't exist.
        /// If the configuration value isn't a legal number, this function will throw an error.
        /// </summary>
        public Output<int>? GetSecretInt32(string key, Int32ConfigOptions? opts = null)
        {
            var v = this.GetInt32(key, opts);
            if (v == null) {
                return null;
            }

            return MakeSecret(v);
        }

        //    /**
        //     * getObject loads an optional configuration value, as an object, by its key, or null if it doesn't exist.
        //     * This routine simply JSON parses and doesn't validate the shape of the contents.
        //     *
        //     * @param key The key to lookup.
        //     */
        //    public getObject<T>(string key): T | null {
        //        var v: string | null = this.Get(key);
        //        if (v == null) {
        //    return null;
        //}
        //        try {
        //    return < T > JSON.parse(v);
        //}
        //        catch (err) {
        //    throw new ConfigTypeError(this.FullKey(key), v, "JSON object");
        //}
        //}

        //*
        // * getSecretObject loads an optional configuration value, as an object, by its key, marking it as a secret
        // * or null if it doesn't exist.
        // * This routine simply JSON parses and doesn't validate the shape of the contents.
        // *
        // * @param key The key to lookup.
        // */
        //public getSecretObject<T>(string key): Output<T> | null {
        //        var v = this.GetObject<T>(key);

        //        if (v == null) {
        //            return null;
        //        }

        //        return MakeSecret<T>(v);
        //    }

        /*
         * Require loads a configuration value by its given key.  If it doesn't exist, an error is thrown.
         *
         * @param key The key to lookup.
         * @param opts An options bag to constrain legal values.
         */
        /// <summary>
        /// Loads a configuration value by its given key.  If it doesn't exist, an error is thrown.
        /// </summary>
        public string Require(string key, StringConfigOptions? opts = null) {
            var v = this.Get(key, opts);
            if (v == null) {
                throw new ConfigMissingError(this.FullKey(key));
            }
            return v;
        }

        /// <summary>
        /// loads a configuration value by its given key, marking it as a secet.  If it doesn't exist, an error
        /// is thrown.
        /// </summary>
        public Output<string> RequireSecret(string key, StringConfigOptions? opts = null) {
            return MakeSecret(this.Require(key, opts));
        }

        /*
         * RequireBool loads a configuration value, as a boolean, by its given key.  If it doesn't exist, or the
         * configuration value is not a legal boolean, an error is thrown.
         *
         * @param key The key to lookup.
         */
        /// <summary>
        /// loads a configuration value, as a boolean, by its given key.  If it doesn't exist, or the
        /// configuration value is not a legal boolean, an error is thrown.
        /// </summary>
        public bool RequireBool(string key) {
            var v = this.GetBool(key);
            if (v == null) {
                throw new ConfigMissingError(this.FullKey(key));
            }
            return v.Value;
        }

        /// <summary>
        /// loads a configuration value, as a boolean, by its given key, marking it as a secret.
        /// If it doesn't exist, or the configuration value is not a legal boolean, an error is thrown.
        /// </summary>
        public Output<bool> RequireSecretBool(string key) {
            return MakeSecret(this.RequireBool(key));
        }

        /*
         * RequireInt32 
         *
         * @param key The key to lookup.
         * @param opts An options bag to constrain legal values.
         */
        /// <summary>
        /// loads a configuration value, as a number, by its given key.  If it doesn't exist, or the
        /// configuration value is not a legal number, an error is thrown.
        /// </summary>
        public int RequireInt32(string key, Int32ConfigOptions? opts = null) {
            var v = this.GetInt32(key, opts);
            if (v == null) {
                throw new ConfigMissingError(this.FullKey(key));
            }
            return v.Value;
        }

        /// <summary>
        /// loads a configuration value, as a number, by its given key, marking it as a secret.
        /// If it doesn't exist, or the configuration value is not a legal number, an error is thrown.
        /// </summary>
        public Output<int> RequireSecretInt32(string key, Int32ConfigOptions? opts = null) {
            return MakeSecret(this.RequireInt32(key, opts));
        }

        //    /*
        //     * RequireObject loads a configuration value as a JSON string and deserializes the JSON into a JavaScript object. If
        //     * it doesn't exist, or the configuration value is not a legal JSON string, an error is thrown.
        //     *
        //     * @param key The key to lookup.
        //     */
        //    public RequireObject<T>(string key): T {
        //        var v: T | null = this.GetObject<T>(key);
        //        if (v == null) {
        //    throw new ConfigMissingError(this.FullKey(key));
        //}
        //        return v;
        //}

        // **
        // * RequireSecretObject loads a configuration value as a JSON string and deserializes the JSON into a JavaScript
        // * object, marking it as a secret. If it doesn't exist, or the configuration value is not a legal JSON
        // * string, an error is thrown.
        // *
        // * @param key The key to lookup.
        // */
        //public RequireSecretObject<T>(string key): Output<T> {
        //        return MakeSecret(this.RequireObject<T>(key));
        //    }

        /// <summary>
        /// turns a simple configuration key into a fully resolved one, by prepending the bag's name.
        /// </summary>
        private string FullKey(string key) {
            return $"{this.name}:{key}";
        }
    }

    /// <summary>
    /// StringConfigOptions may be used to constrain the set of legal values a string config value may contain.
    /// </summary>
    public class StringConfigOptions
    {
        /// <summary>
        /// The legal enum values. If it does not match, a ConfigEnumError is thrown.
        /// </summary>
        public ISet<string> AllowedValues = new HashSet<string>();

        /// <summary>
        /// The minimum string length. If the string is not this long, a ConfigRangeError is thrown.
        /// </summary>
        public int? MinLength;

        /// <summary>
        /// The maximum string length. If the string is longer than this, a ConfigRangeError is thrown.
        /// </summary>
        public int? MaxLength;

        /// <summary>
        /// A regular expression the string must match. If it does not match, a ConfigPatternError is thrown.
        /// </summary>
        public Regex? Pattern;
    }

    /// <summary>
    /// Int32ConfigOptions may be used to constrain the set of legal values a number config value may contain.
    /// </summary>
    public class Int32ConfigOptions
    {
        /// <summary>
        /// The minimum number value, inclusive. If the number is less than this, a ConfigRangeError
        /// is thrown.
        /// </summary>
        int? Min;

        /// <summary>
        /// The maximum number value, inclusive. If the number is greater than this, a
        /// ConfigRangeError is thrown.
        /// </summary>
        int? Max;
    }

    /// <summary>
    /// ConfigTypeError is used when a configuration value is of the wrong type.
    /// </summary>
    internal class ConfigTypeException : RunException
    {
        public ConfigTypeException(string key, object v, string expectedType)
            : base($"Configuration '{key}' value '{v}' is not a valid {expectedType}")
        {
        }
    }

    /// <summary>
    /// ConfigEnumError is used when a configuration value isn't a correct enum value.
    /// </summary>
    internal class ConfigEnumError : RunException
    {
        public ConfigEnumError(string key, object v, ISet<string> values)
            : base($"Configuration '{key}' value '{v}' is not a legal enum value ({new JArray(values)}")
        {
        }
    }

    /// <summary>
    /// ConfigRangeError is used when a configuration value is outside of the range of legal sizes.
    /// </summary>
    internal class ConfigRangeError : RunException
    {
        public ConfigRangeError(string key, object v, int? min, int? max)
        : base($"Configuration '{key}' value '{v}' is outside of the legal range({GetRange(v, min, max)}, inclusive)")
        {
        }

        private static object GetRange(object v, int? min, int? max)
        {
            string range;
            if (max == null)
            {
                range = $"min {min}";
            }
            else if (min == null)
            {
                range = $"max {max}";
            }
            else
            {
                range = $"{min}-{max}";
            }

            if (v is string)
            {
                range += " chars";
            }

            return range;
        }
    }

    /// <summary>
    /// ConfigPatternError is used when a configuration value does not match the given regular expression.
    /// </summary>
    internal class ConfigPatternError : RunException
    {
        public ConfigPatternError(string key, string v, Regex pattern)
                : base($"Configuration '{key}' value '{v}' does not match the regular expression '{pattern}'")
        {
        }
    }

    /// <summary>
    /// ConfigMissingError is used when a configuration value is completely missing.
    /// </summary>
    internal class ConfigMissingError : RunException
    {
        public ConfigMissingError(string key)
                : base($"Missing Required configuration variable '{key}'\n" +
                $"\tplease set a value using the command `pulumi config set ${key} <value>`")
        {
        }
    }
}
