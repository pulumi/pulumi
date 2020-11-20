using System.Text.Json;
using YamlDotNet.Serialization;

namespace Pulumi.X.Automation.Serialization
{
    internal class LocalSerializer
    {
        private readonly IDeserializer _yamlDeserializer;
        private readonly ISerializer _yamlSerializer;

        public LocalSerializer()
        {
            this._yamlDeserializer = new DeserializerBuilder().Build();
            this._yamlSerializer = new SerializerBuilder().Build();
        }

        public T DeserializeJson<T>(string content)
            where T : class
            => JsonSerializer.Deserialize<T>(content);

        public T DeserializeYaml<T>(string content)
            where T : class
            => this._yamlDeserializer.Deserialize<T>(content);

        public string SerializeJson<T>(T @object)
            where T : class
            => JsonSerializer.Serialize(@object);

        public string SerializeYaml<T>(T @object)
            where T : class
            => this._yamlSerializer.Serialize(@object);
    }
}
