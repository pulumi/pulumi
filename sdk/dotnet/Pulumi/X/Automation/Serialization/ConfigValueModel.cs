using Pulumi.X.Automation.Serialization.Json;

namespace Pulumi.X.Automation.Serialization
{
    // necessary because this version of System.Text.Json
    // can't deserialize a type that doesn't have a parameterless constructor
    internal class ConfigValueModel : IJsonModel<ConfigValue>
    {
        public string Value { get; set; } = null!;

        public bool IsSecret { get; set; }

        public ConfigValue Convert()
            => new ConfigValue(this.Value, this.IsSecret);
    }
}
