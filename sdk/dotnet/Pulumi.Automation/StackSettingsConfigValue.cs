// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    public class StackSettingsConfigValue
    {
        public string Value { get; }

        public bool IsSecure { get; }

        public StackSettingsConfigValue(
            string value,
            bool isSecure)
        {
            this.Value = value;
            this.IsSecure = isSecure;
        }
    }
}
