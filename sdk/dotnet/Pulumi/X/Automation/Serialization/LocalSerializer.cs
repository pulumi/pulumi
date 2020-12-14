using System.Text.Json;
using System.Text.Json.Serialization;
using Pulumi.X.Automation.Serialization.Json;
using YamlDotNet.Serialization;

namespace Pulumi.X.Automation.Serialization
{
    internal class LocalSerializer
    {
        private readonly JsonSerializerOptions _jsonOptions;
        private readonly IDeserializer _yamlDeserializer;
        private readonly ISerializer _yamlSerializer;

        public LocalSerializer()
        {
            // configure json
            this._jsonOptions = new JsonSerializerOptions
            {
                PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
            };

            this._jsonOptions.Converters.Add(new JsonStringEnumConverter(new LowercaseNamingPolicy()));
            this._jsonOptions.Converters.Add(new ProjectSettingsConverter());
            this._jsonOptions.Converters.Add(new ProjectRuntimeConverter());
            this._jsonOptions.Converters.Add(new StackSettingsConfigValueConverter());

            // configure yaml
            this._yamlDeserializer = new DeserializerBuilder().Build();
            this._yamlSerializer = new SerializerBuilder().Build();
        }

        public T DeserializeJson<T>(string content)
            where T : class
            => JsonSerializer.Deserialize<T>(content, this._jsonOptions);

        public T DeserializeYaml<T>(string content)
            where T : class
            => this._yamlDeserializer.Deserialize<T>(content);

        public string SerializeJson<T>(T @object)
            where T : class
            => JsonSerializer.Serialize(@object, this._jsonOptions);

        public string SerializeYaml<T>(T @object)
            where T : class
            => this._yamlSerializer.Serialize(@object);
    }
}
