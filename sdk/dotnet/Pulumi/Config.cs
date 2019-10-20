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
    public partial class Config
    {
        /// <summary>
        /// name is the configuration bag's logical name and uniquely identifies it.  The default
        /// is the name of the current project.
        /// </summary>
        private readonly string _name;

        public Config(string? name = null)
        {
            if (name == null)
            {
                name = Deployment.Instance.Options.Project;
            }

            if (name.EndsWith(":config", StringComparison.Ordinal))
            {
                name = name[0..^":config".Length];
            }

            this._name = name;
        }

        private static Output<T> MakeSecret<T>(T value)
        {
            return new Output<T>(
                ImmutableHashSet<Resource>.Empty,
                Task.FromResult(new OutputData<T>(value, isKnown: true, isSecret: true)));
        }

        /// <summary>
        /// Loads an optional configuration value by its key, or <see langword="null"/> if it doesn't exist.
        /// </summary>
        public string? Get(string key, StringOptions? opts = null)
        {
            var v = Deployment.Instance.GetConfig(this.FullKey(key));
            if (v == null)
            {
                return null;
            }

            if (opts != null)
            {
                // SAFETY: if allowedValues != null, verifying v ∈ K[]
                if (opts.AllowedValues.Count > 0 && !opts.AllowedValues.Contains(v))
                {
                    throw new ConfigEnumException(this.FullKey(key), v, opts.AllowedValues);
                }
                else if (opts.MinLength != null && v.Length < opts.MinLength)
                {
                    throw new ConfigRangeException(this.FullKey(key), v, opts.MinLength, null);
                }
                else if (opts.MaxLength != null && v.Length > opts.MaxLength)
                {
                    throw new ConfigRangeException(this.FullKey(key), v, null, opts.MaxLength);
                }
                else if (opts.Pattern != null)
                {
                    var pattern = opts.Pattern;
                    if (!pattern.IsMatch(v))
                    {
                        throw new ConfigPatternException(this.FullKey(key), v, pattern);
                    }
                }
            }

            return v;
        }

        /// <summary>
        /// Loads an optional configuration value by its key, marking it as a secret, or <see
        /// langword="null"/> if it doesn't exist.
        /// </summary>
        public Output<string>? GetSecret(string key, StringOptions? opts = null)
        {
            var v = this.Get(key, opts);
            return v == null ? null : MakeSecret(v);
        }

        /// <summary>
        /// Loads an optional configuration value, as a boolean, by its key, or null if it doesn't exist.
        /// If the configuration value isn't a legal boolean, this function will throw an error.
        /// </summary>
        public bool? GetBool(string key)
        {
            var v = this.Get(key);
            if (v == null)
            {
                return null;
            }
            else if (v == "true")
            {
                return true;
            }
            else if (v == "false")
            {
                return false;
            }

            throw new ConfigTypeException(this.FullKey(key), v, "bool");
        }

        /// <summary>
        /// loads an optional configuration value, as a boolean, by its key, making it as a secret or
        /// null if it doesn't exist. If the configuration value isn't a legal boolean, this
        /// function will throw an error.
        /// </summary>
        public Output<bool>? GetSecretBool(string key)
        {
            var v = this.GetBool(key);
            return v == null ? null : MakeSecret(v.Value);
        }

        /// <summary>
        /// Loads an optional configuration value, as a number, by its key, or null if it doesn't exist.
        /// If the configuration value isn't a legal number, this function will throw an error.
        /// </summary>
        public int? GetInt32(string key, Int32Options? opts = null)
        {
            var v = this.Get(key);
            if (v == null)
            {
                return null;
            }

            if (!int.TryParse(v, out var result))
            {
                throw new ConfigTypeException(this.FullKey(key), v, "Int32");
            }

            if (opts != null)
            {
                if (opts.Min != null && result < opts.Min)
                {
                    throw new ConfigRangeException(this.FullKey(key), result, opts.Min, null);
                }
                else if (opts.Max != null && result > opts.Max)
                {
                    throw new ConfigRangeException(this.FullKey(key), result, null, opts.Max);
                }
            }

            return result;
        }

        /// <summary>
        /// loads an optional configuration value, as a number, by its key, marking it as a secret
        /// or null if it doesn't exist.
        /// If the configuration value isn't a legal number, this function will throw an error.
        /// </summary>
        public Output<int>? GetSecretInt32(string key, Int32Options? opts = null)
        {
            var v = this.GetInt32(key, opts);
            return v == null ? null : MakeSecret(v.Value);
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
        //    throw new ConfigTypeException(this.FullKey(key), v, "JSON object");
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

        /// <summary>
        /// Loads a configuration value by its given key.  If it doesn't exist, an error is thrown.
        /// </summary>
        public string Require(string key, StringOptions? opts = null)
            => this.Get(key, opts) ?? throw new ConfigMissingException(this.FullKey(key));

        /// <summary>
        /// loads a configuration value by its given key, marking it as a secet.  If it doesn't exist, an error
        /// is thrown.
        /// </summary>
        public Output<string> RequireSecret(string key, StringOptions? opts = null)
            => MakeSecret(this.Require(key, opts));

        /// <summary>
        /// loads a configuration value, as a boolean, by its given key.  If it doesn't exist, or the
        /// configuration value is not a legal boolean, an error is thrown.
        /// </summary>
        public bool RequireBool(string key)
            => this.GetBool(key) ?? throw new ConfigMissingException(this.FullKey(key));

        /// <summary>
        /// loads a configuration value, as a boolean, by its given key, marking it as a secret.
        /// If it doesn't exist, or the configuration value is not a legal boolean, an error is thrown.
        /// </summary>
        public Output<bool> RequireSecretBool(string key)
            => MakeSecret(this.RequireBool(key));

        /// <summary>
        /// loads a configuration value, as a number, by its given key.  If it doesn't exist, or the
        /// configuration value is not a legal number, an error is thrown.
        /// </summary>
        public int RequireInt32(string key, Int32Options? opts = null)
            => this.GetInt32(key, opts) ?? throw new ConfigMissingException(this.FullKey(key));

        /// <summary>
        /// loads a configuration value, as a number, by its given key, marking it as a secret.
        /// If it doesn't exist, or the configuration value is not a legal number, an error is thrown.
        /// </summary>
        public Output<int> RequireSecretInt32(string key, Int32Options? opts = null)
            => MakeSecret(this.RequireInt32(key, opts));

        //    /*
        //     * RequireObject loads a configuration value as a JSON string and deserializes the JSON into a JavaScript object. If
        //     * it doesn't exist, or the configuration value is not a legal JSON string, an error is thrown.
        //     *
        //     * @param key The key to lookup.
        //     */
        //    public RequireObject<T>(string key): T {
        //        var v: T | null = this.GetObject<T>(key);
        //        if (v == null) {
        //    throw new ConfigMissingException(this.FullKey(key));
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
        private string FullKey(string key)
            => $"{this._name}:{key}";
    }
}