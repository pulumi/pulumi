// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Diagnostics.CodeAnalysis;
using System.Runtime.CompilerServices;
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
            name ??= Deployment.Instance.ProjectName;
            if (name.EndsWith(":config", StringComparison.Ordinal))
            {
                name = name[..^":config".Length];
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


        private string? GetImpl(string key, string? use = null, [CallerMemberName] string? insteadOf = null)
        {
            var fullKey = FullKey(key);
            // TODO[pulumi/pulumi#7127]: Re-enable the warning.
            // if (use != null && Deployment.InternalInstance.IsConfigSecret(fullKey))
            // {
            //     Debug.Assert(insteadOf != null);
            //     Log.Warn($"Configuration '{fullKey}' value is a secret; use `{use}` instead of `{insteadOf}`");
            // }
            return Deployment.InternalInstance.GetConfig(fullKey);
        }

        /// <summary>
        /// Loads an optional configuration value by its key, or <see langword="null"/> if it doesn't exist.
        /// </summary>
        public string? Get(string key)
            => GetImpl(key, nameof(GetSecret));

        /// <summary>
        /// Loads an optional configuration value by its key, marking it as a secret, or <see
        /// langword="null"/> if it doesn't exist.
        /// </summary>
        public Output<string>? GetSecret(string key)
            => MakeClassSecret(GetImpl(key));

        private bool? GetBooleanImpl(string key, string? use = null, [CallerMemberName] string? insteadOf = null)
        {
            var v = GetImpl(key, use, insteadOf);
            return v switch
            {
                null => default(bool?),
                "true" => true,
                "false" => false,
                _ => throw new ConfigTypeException(FullKey(key), v, nameof(Boolean))
            };
        }

        /// <summary>
        /// Loads an optional configuration value, as a boolean, by its key, or null if it doesn't exist.
        /// If the configuration value isn't a legal boolean, this function will throw an error.
        /// </summary>
        public bool? GetBoolean(string key)
            => GetBooleanImpl(key, nameof(GetSecretBoolean));

        /// <summary>
        /// Loads an optional configuration value, as a boolean, by its key, making it as a secret or
        /// null if it doesn't exist. If the configuration value isn't a legal boolean, this
        /// function will throw an error.
        /// </summary>
        public Output<bool>? GetSecretBoolean(string key)
            => MakeStructSecret(GetBooleanImpl(key));

        private int? GetInt32Impl(string key, string? use = null, [CallerMemberName] string? insteadOf = null)
        {
            var v = GetImpl(key, use, insteadOf);
            return v == null
                ? default(int?)
                : int.TryParse(v, out var result)
                    ? result
                    : throw new ConfigTypeException(FullKey(key), v, nameof(Int32));
        }

        /// <summary>
        /// Loads an optional configuration value, as a number, by its key, or null if it doesn't exist.
        /// If the configuration value isn't a legal number, this function will throw an error.
        /// </summary>
        public int? GetInt32(string key)
            => GetInt32Impl(key, nameof(GetSecretInt32));

        /// <summary>
        /// Loads an optional configuration value, as a number, by its key, marking it as a secret
        /// or null if it doesn't exist.
        /// If the configuration value isn't a legal number, this function will throw an error.
        /// </summary>
        public Output<int>? GetSecretInt32(string key)
            => MakeStructSecret(GetInt32Impl(key));

        [return: MaybeNull]
        private T GetObjectImpl<T>(string key, string? use = null, [CallerMemberName] string? insteadOf = null)
        {
            var v = GetImpl(key, use, insteadOf);
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
        /// Loads an optional configuration value, as an object, by its key, or null if it doesn't
        /// exist. This works by taking the value associated with <paramref name="key"/> and passing
        /// it to <see cref="JsonSerializer.Deserialize{TValue}(string, JsonSerializerOptions)"/>.
        /// </summary>
        [return: MaybeNull]
        public T GetObject<T>(string key)
            => GetObjectImpl<T>(key, nameof(GetSecretObject));

        /// <summary>
        /// Loads an optional configuration value, as an object, by its key, marking it as a secret
        /// or null if it doesn't exist. This works by taking the value associated with <paramref
        /// name="key"/> and passing it to <see cref="JsonSerializer.Deserialize{TValue}(string, JsonSerializerOptions)"/>.
        /// </summary>
        public Output<T>? GetSecretObject<T>(string key)
        {
            var v = GetImpl(key);
            if (v == null)
                return null;

            return Output.CreateSecret(GetObjectImpl<T>(key)!);
        }

        private string RequireImpl(string key, string? use = null, [CallerMemberName] string? insteadOf = null)
            => GetImpl(key, use, insteadOf) ?? throw new ConfigMissingException(FullKey(key));

        /// <summary>
        /// Loads a configuration value by its given key.  If it doesn't exist, an error is thrown.
        /// </summary>
        public string Require(string key)
            => RequireImpl(key, nameof(RequireSecret));

        /// <summary>
        /// Loads a configuration value by its given key, marking it as a secret.  If it doesn't exist, an error
        /// is thrown.
        /// </summary>
        public Output<string> RequireSecret(string key)
            => MakeClassSecret(RequireImpl(key));

        private bool RequireBooleanImpl(string key, string? use = null, [CallerMemberName] string? insteadOf = null)
            => GetBooleanImpl(key, use, insteadOf) ?? throw new ConfigMissingException(FullKey(key));

        /// <summary>
        /// Loads a configuration value, as a boolean, by its given key.  If it doesn't exist, or the
        /// configuration value is not a legal boolean, an error is thrown.
        /// </summary>
        public bool RequireBoolean(string key)
            => RequireBooleanImpl(key, nameof(RequireSecretBoolean));

        /// <summary>
        /// Loads a configuration value, as a boolean, by its given key, marking it as a secret.
        /// If it doesn't exist, or the configuration value is not a legal boolean, an error is thrown.
        /// </summary>
        public Output<bool> RequireSecretBoolean(string key)
            => MakeStructSecret(RequireBooleanImpl(key));

        private int RequireInt32Impl(string key, string? use = null, [CallerMemberName] string? insteadOf = null)
            => GetInt32Impl(key, use, insteadOf) ?? throw new ConfigMissingException(FullKey(key));

        /// <summary>
        /// Loads a configuration value, as a number, by its given key.  If it doesn't exist, or the
        /// configuration value is not a legal number, an error is thrown.
        /// </summary>
        public int RequireInt32(string key)
            => RequireInt32Impl(key, nameof(RequireSecretInt32));

        /// <summary>
        /// Loads a configuration value, as a number, by its given key, marking it as a secret.
        /// If it doesn't exist, or the configuration value is not a legal number, an error is thrown.
        /// </summary>
        public Output<int> RequireSecretInt32(string key)
            => MakeStructSecret(RequireInt32Impl(key));

        private T RequireObjectImpl<T>(string key, string? use = null, [CallerMemberName] string? insteadOf = null)
        {
            var v = GetImpl(key);
            if (v == null)
                throw new ConfigMissingException(FullKey(key));

            return GetObjectImpl<T>(key, use, insteadOf)!;
        }

        /// <summary>
        /// Loads a configuration value as a JSON string and deserializes the JSON into an object.
        /// object. If it doesn't exist, or the configuration value cannot be converted using <see
        /// cref="JsonSerializer.Deserialize{TValue}(string, JsonSerializerOptions)"/>, an error is
        /// thrown.
        /// </summary>
        public T RequireObject<T>(string key)
            => RequireObjectImpl<T>(key, nameof(RequireSecretObject));

        /// <summary>
        /// Loads a configuration value as a JSON string and deserializes the JSON into a JavaScript
        /// object, marking it as a secret. If it doesn't exist, or the configuration value cannot
        /// be converted using <see cref="JsonSerializer.Deserialize{TValue}(string, JsonSerializerOptions)"/>,
        /// an error is thrown.
        /// </summary>
        public Output<T> RequireSecretObject<T>(string key)
            => Output.CreateSecret(RequireObjectImpl<T>(key));

        /// <summary>
        /// Turns a simple configuration key into a fully resolved one, by prepending the bag's name.
        /// </summary>
        private string FullKey(string key)
            => $"{_name}:{key}";
    }
}
