// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Text.Json;

namespace Pulumi
{
    public partial class Deployment
    {
        /// <summary>
        /// The environment variable key that the language plugin uses to set configuration values.
        /// </summary>
        private const string _configEnvKey = "PULUMI_CONFIG";

        /// <summary>
        /// The environment variable key that the language plugin uses to set the list of secret configuration keys.
        /// </summary>
        private const string _configSecretKeysEnvKey = "PULUMI_CONFIG_SECRET_KEYS";

        /// <summary>
        /// Returns a copy of the full config map.
        /// </summary>
        internal ImmutableDictionary<string, string> AllConfig { get; private set; } = ParseConfig();

        /// <summary>
        /// Returns a copy of the config secret keys.
        /// </summary>
        internal ImmutableHashSet<string> ConfigSecretKeys { get; private set; } = ParseConfigSecretKeys();

        /// <summary>
        /// Sets a configuration variable.
        /// </summary>
        internal void SetConfig(string key, string value)
            => AllConfig = AllConfig.Add(key, value);

        /// <summary>
        /// Appends all provided configuration.
        /// </summary>
        internal void SetAllConfig(IDictionary<string, string> config, IEnumerable<string>? secretKeys = null)
        {
            AllConfig = AllConfig.AddRange(config);
            if (secretKeys != null)
            {
                ConfigSecretKeys = ConfigSecretKeys.Union(secretKeys);
            }
        }

        /// <summary>
        /// Returns a configuration variable's value or <see langword="null"/> if it is unset.
        /// </summary>
        string? IDeploymentInternal.GetConfig(string key)
            => AllConfig.TryGetValue(key, out var value) ? value : null;

        /// <summary>
        /// Returns true if the key contains a secret value.
        /// </summary>
        bool IDeploymentInternal.IsConfigSecret(string fullKey)
            => ConfigSecretKeys.Contains(fullKey);

        private static ImmutableDictionary<string, string> ParseConfig()
        {
            var parsedConfig = ImmutableDictionary.CreateBuilder<string, string>();
            var envConfig = Environment.GetEnvironmentVariable(_configEnvKey);

            if (envConfig != null)
            {
                var envObject = JsonDocument.Parse(envConfig);
                foreach (var prop in envObject.RootElement.EnumerateObject())
                {
                    parsedConfig[CleanKey(prop.Name)] = prop.Value.ToString();
                }
            }

            return parsedConfig.ToImmutable();
        }

        private static ImmutableHashSet<string> ParseConfigSecretKeys()
        {
            var parsedConfigSecretKeys = ImmutableHashSet.CreateBuilder<string>();
            var envConfigSecretKeys = Environment.GetEnvironmentVariable(_configSecretKeysEnvKey);

            if (envConfigSecretKeys != null)
            {
                var envObject = JsonDocument.Parse(envConfigSecretKeys);
                foreach (var element in envObject.RootElement.EnumerateArray())
                {
                    parsedConfigSecretKeys.Add(element.GetString());
                }
            }

            return parsedConfigSecretKeys.ToImmutable();
        }

        /// <summary>
        /// CleanKey takes a configuration key, and if it is of the form "(string):config:(string)"
        /// removes the ":config:" portion. Previously, our keys always had the string ":config:" in
        /// them, and we'd like to remove it. However, the language host needs to continue to set it
        /// so we can be compatible with older versions of our packages. Once we stop supporting
        /// older packages, we can change the language host to not add this :config: thing and
        /// remove this function.
        /// </summary>
        private static string CleanKey(string key)
        {
            var idx = key.IndexOf(":", StringComparison.Ordinal);

            if (idx > 0 && key.Substring(idx + 1).StartsWith("config:", StringComparison.Ordinal))
            {
                return key.Substring(0, idx) + ":" + key.Substring(idx + 1 + "config:".Length);
            }

            return key;
        }
    }
}
