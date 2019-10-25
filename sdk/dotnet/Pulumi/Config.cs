// Copyright 2016-2018, Pulumi Corporation

using System;
using System.Diagnostics.CodeAnalysis;
using System.Text.Json;

namespace Pulumi
{
    /// <summary>
    /// <see cref="Config"/> is a bag of related configuration state.  Each bag contains any number
    /// of configuration variables, indexed by simple keys, and each has a name that uniquely
    /// identifies it; two bags with different names do not share values for variables that
    /// otherwise share the same key.  For example, a bag whose name is <c>pulumi:foo</c>, with keys
    /// <c>a</c>, <c>b</c>, and <c>c</c>, is entirely separate from a bag whose name is
    /// <c>pulumi:bar</c> with the same simple key names.  Each key has a fully qualified names,
    /// such as <c>pulumi:foo:a</c>, ..., and <c>pulumi:bar:a</c>, respectively.
    /// </summary>
    public sealed partial class Config
    {
        /// <summary>
        /// <see cref="_name"/> is the configuration bag's logical name and uniquely identifies it.
        /// The default is the name of the current project.
        /// </summary>
        private readonly string _name;

        /// <summary>
        /// Creates a new <see cref="Config"/> instance. <paramref name="name"/> is the
        /// configuration bag's logical name and uniquely identifies it. The default is the name of
        /// the current project.
        /// </summary>
        public Config(string? name = null)
        {
            if (name == null)
            {
                name = Deployment.Instance.ProjectName;
            }

            if (name.EndsWith(":config", StringComparison.Ordinal))
            {
                name = name[0..^":config".Length];
            }

            _name = name;
        }

        [return: NotNullIfNotNull("value")]
        private static Output<T>? MakeClassSecret<T>(T? value) where T : class
            => value == null ? null : Output.CreateSecret(value);

        private static Output<T>? MakeStructSecret<T>(T? value) where T : struct
            => value == null ? null : MakeStructSecret(value.Value);

        private static Output<T> MakeStructSecret<T>(T value) where T : struct
            => Output.CreateSecret(value);


        /// <summary>
        /// Loads an optional configuration value by its key, or <see langword="null"/> if it doesn't exist.
        /// </summary>
        public string? Get(string key)
            => Deployment.InternalInstance.GetConfig(FullKey(key));

        /// <summary>
        /// Loads an optional configuration value by its key, marking it as a secret, or <see
        /// langword="null"/> if it doesn't exist.
        /// </summary>
        public Output<string>? GetSecret(string key)
            => MakeClassSecret(Get(key));

        /// <summary>
        /// Loads an optional configuration value, as a boolean, by its key, or null if it doesn't exist.
        /// If the configuration value isn't a legal boolean, this function will throw an error.
        /// </summary>
        public bool? GetBoolean(string key)
        {
            var v = Get(key);
            return v == null ? default(bool?) :
                   v == "true" ? true :
                   v == "false" ? false : throw new ConfigTypeException(FullKey(key), v, nameof(Boolean));
        }

        /// <summary>
        /// Loads an optional configuration value, as a boolean, by its key, making it as a secret or
        /// null if it doesn't exist. If the configuration value isn't a legal boolean, this
        /// function will throw an error.
        /// </summary>
        public Output<bool>? GetSecretBoolean(string key)
            => MakeStructSecret(GetBoolean(key));

        /// <summary>
        /// Loads an optional configuration value, as a number, by its key, or null if it doesn't exist.
        /// If the configuration value isn't a legal number, this function will throw an error.
        /// </summary>
        public int? GetInt32(string key)
        {
            var v = Get(key);
            return v == null
                ? default(int?)
                : int.TryParse(v, out var result)
                    ? result
                    : throw new ConfigTypeException(FullKey(key), v, nameof(Int32));
        }

        /// <summary>
        /// Loads an optional configuration value, as a number, by its key, marking it as a secret
        /// or null if it doesn't exist.
        /// If the configuration value isn't a legal number, this function will throw an error.
        /// </summary>
        public Output<int>? GetSecretInt32(string key)
            => MakeStructSecret(GetInt32(key));

        /// <summary>
        /// Loads an optional configuration value, as an object, by its key, or null if it doesn't
        /// exist. This works by taking the value associated with <paramref name="key"/> and passing
        /// it to <see cref="JsonSerializer.Deserialize{TValue}(string, JsonSerializerOptions)"/>.
        /// </summary>
        [return: MaybeNull]
        public T GetObject<T>(string key)
        {
            var v = Get(key);
            try
            {
                return v == null ? default : JsonSerializer.Deserialize<T>(v);
            }
            catch (JsonException ex)
            {
                throw new ConfigTypeException(FullKey(key), v, typeof(T).FullName!, ex);
            }
        }

        /// <summary>
        /// Loads an optional configuration value, as an object, by its key, marking it as a secret
        /// or null if it doesn't exist. This works by taking the value associated with <paramref
        /// name="key"/> and passing it to <see cref="JsonSerializer.Deserialize{TValue}(string,
        /// JsonSerializerOptions)"/>.
        /// </summary>
        public Output<T>? GetSecretObject<T>(string key)
        {
            var v = Get(key);
            if (v == null)
                return null;

            return Output.CreateSecret(GetObject<T>(key)!);
        }

        /// <summary>
        /// Loads a configuration value by its given key.  If it doesn't exist, an error is thrown.
        /// </summary>
        public string Require(string key)
            => Get(key) ?? throw new ConfigMissingException(FullKey(key));

        /// <summary>
        /// Loads a configuration value by its given key, marking it as a secret.  If it doesn't exist, an error
        /// is thrown.
        /// </summary>
        public Output<string> RequireSecret(string key)
            => MakeClassSecret(Require(key));

        /// <summary>
        /// Loads a configuration value, as a boolean, by its given key.  If it doesn't exist, or the
        /// configuration value is not a legal boolean, an error is thrown.
        /// </summary>
        public bool RequireBoolean(string key)
            => GetBoolean(key) ?? throw new ConfigMissingException(FullKey(key));

        /// <summary>
        /// Loads a configuration value, as a boolean, by its given key, marking it as a secret.
        /// If it doesn't exist, or the configuration value is not a legal boolean, an error is thrown.
        /// </summary>
        public Output<bool> RequireSecretBoolean(string key)
            => MakeStructSecret(RequireBoolean(key));

        /// <summary>
        /// Loads a configuration value, as a number, by its given key.  If it doesn't exist, or the
        /// configuration value is not a legal number, an error is thrown.
        /// </summary>
        public int RequireInt32(string key)
            => GetInt32(key) ?? throw new ConfigMissingException(FullKey(key));

        /// <summary>
        /// Loads a configuration value, as a number, by its given key, marking it as a secret.
        /// If it doesn't exist, or the configuration value is not a legal number, an error is thrown.
        /// </summary>
        public Output<int> RequireSecretInt32(string key)
            => MakeStructSecret(RequireInt32(key));

        /// <summary>
        /// Loads a configuration value as a JSON string and deserializes the JSON into an object.
        /// object. If it doesn't exist, or the configuration value cannot be converted using <see
        /// cref="JsonSerializer.Deserialize{TValue}(string, JsonSerializerOptions)"/>, an error is
        /// thrown.
        /// </summary>
        public T RequireObject<T>(string key)
        {
            var v = Get(key);
            if (v == null)
                throw new ConfigMissingException(FullKey(key));

            return GetObject<T>(key)!;
        }

        /// <summary>
        /// Loads a configuration value as a JSON string and deserializes the JSON into a JavaScript
        /// object, marking it as a secret. If it doesn't exist, or the configuration value cannot
        /// be converted using <see cref="JsonSerializer.Deserialize{TValue}(string,
        /// JsonSerializerOptions)"/>. an error is thrown.
        /// </summary>
        public Output<T> RequireSecretObject<T>(string key)
            => Output.CreateSecret(RequireObject<T>(key));

        /// <summary>
        /// Turns a simple configuration key into a fully resolved one, by prepending the bag's name.
        /// </summary>
        private string FullKey(string key)
            => $"{_name}:{key}";
    }
}
