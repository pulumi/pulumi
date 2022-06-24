// Copyright 2016-2019, Pulumi Corporation

using System;

namespace Pulumi
{
    public partial class Config
    {
        /// <summary>
        /// ConfigTypeException is used when a configuration value is of the wrong type.
        /// </summary>
        private class ConfigTypeException : RunException
        {
            public ConfigTypeException(string key, object? v, string expectedType, Exception? innerException = null)
                : base($"Configuration '{key}' value '{v}' is not a valid {expectedType}", innerException)
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
