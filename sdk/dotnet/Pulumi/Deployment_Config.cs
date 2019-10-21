// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using Newtonsoft.Json.Linq;

namespace Pulumi
{
    public partial class Deployment
    {
        /// <summary>
        /// The environment variable key that the language plugin uses to set configuration values.
        /// </summary>
        private const string _configEnvKey = "PULUMI_CONFIG";

        private readonly object _configGate = new object();
        private readonly Dictionary<string, string> _config = ParseConfig();

        /// <summary>
        /// returns a copy of the full config map.
        /// </summary>
        internal ImmutableDictionary<string, string> AllConfig()
        {
            lock (_configGate)
            {
                return _config.ToImmutableDictionary();
            }
        }

        /// <summary>
        /// sets a configuration variable.
        /// </summary>
        internal void SetConfig(string key, string value)
        {
            lock (_configGate)
            {
                _config[key] = value;
            }
        }

        /// <summary>
        /// returns a configuration variable's value or <see langword="null"/> if it is unset.
        /// </summary>
        string? IDeploymentInternal.GetConfig(string key)
        {
            lock (_configGate)
            {
                return _config.TryGetValue(key, out var value)
                    ? value
                    : null;
            }
        }

        private static Dictionary<string, string> ParseConfig()
        {
            var parsedConfig = new Dictionary<string, string>();
            var envConfig = Environment.GetEnvironmentVariable(_configEnvKey);

            if (envConfig != null)
            {
                var envObject = JObject.Parse(envConfig);
                foreach (var prop in envObject.Properties())
                {
                    parsedConfig[CleanKey(prop.Name)] = prop.Value.ToString();
                }
            }

            return parsedConfig;
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
