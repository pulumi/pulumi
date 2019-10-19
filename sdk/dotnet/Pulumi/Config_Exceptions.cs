// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using System.Text.RegularExpressions;
using Newtonsoft.Json.Linq;

namespace Pulumi
{
    public partial class Config
    {
        /// <summary>
        /// ConfigTypeException is used when a configuration value is of the wrong type.
        /// </summary>
        private class ConfigTypeException : RunException
        {
            public ConfigTypeException(string key, object v, string expectedType)
                : base($"Configuration '{key}' value '{v}' is not a valid {expectedType}")
            {
            }
        }

        /// <summary>
        /// ConfigEnumException is used when a configuration value isn't a correct enum value.
        /// </summary>
        private class ConfigEnumException : RunException
        {
            public ConfigEnumException(string key, object v, ISet<string> values)
                : base($"Configuration '{key}' value '{v}' is not a legal enum value ({new JArray(values)}")
            {
            }
        }

        /// <summary>
        /// ConfigRangeException is used when a configuration value is outside of the range of legal sizes.
        /// </summary>
        private class ConfigRangeException : RunException
        {
            public ConfigRangeException(string key, object v, int? min, int? max)
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
        /// ConfigPatternException is used when a configuration value does not match the given regular expression.
        /// </summary>
        private class ConfigPatternException : RunException
        {
            public ConfigPatternException(string key, string v, Regex pattern)
                    : base($"Configuration '{key}' value '{v}' does not match the regular expression '{pattern}'")
            {
            }
        }

        /// <summary>
        /// ConfigMissingException is used when a configuration value is completely missing.
        /// </summary>
        private class ConfigMissingException : RunException
        {
            public ConfigMissingException(string key)
                    : base($"Missing Required configuration variable '{key}'\n" +
                    $"\tplease set a value using the command `pulumi config set {key} <value>`")
            {
            }
        }
    }
}
