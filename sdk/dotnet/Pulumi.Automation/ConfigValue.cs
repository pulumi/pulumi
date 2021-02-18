// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    public class ConfigValue
    {
        public string Value { get; set; }

        public bool IsSecret { get; set; }

        public ConfigValue(
            string value,
            bool isSecret = false)
        {
            this.Value = value;
            this.IsSecret = isSecret;
        }
    }
}
