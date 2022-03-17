// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Serialization.Json;

namespace Pulumi.Automation.Serialization
{
    // necessary for constructor deserialization
    internal class ConfigValueModel : IJsonModel<ConfigValue>
    {
        public string Value { get; set; } = null!;

        public bool Secret { get; set; }

        public ConfigValue Convert()
            => new ConfigValue(this.Value, this.Secret);
    }
}
