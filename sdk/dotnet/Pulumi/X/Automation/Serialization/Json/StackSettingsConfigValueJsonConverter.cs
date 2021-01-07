using System;
using System.Diagnostics.CodeAnalysis;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;

namespace Pulumi.X.Automation.Serialization.Json
{
    internal class StackSettingsConfigValueJsonConverter : JsonConverter<StackSettingsConfigValue>
    {
        public override StackSettingsConfigValue ReadJson(
            JsonReader reader,
            Type objectType,
            [AllowNull] StackSettingsConfigValue existingValue,
            bool hasExistingValue,
            JsonSerializer serializer)
        {
            // check if plain string
            if (reader.TokenType is JsonToken.String
                && reader.Value is string valueStr)
                return new StackSettingsConfigValue(valueStr, false);

            // confirm object
            if (reader.TokenType != JsonToken.StartObject)
                throw new JsonException($"Unable to deserialize [{objectType.FullName}]. Expecting object if not plain string.");

            // parse object
            var element = serializer.Deserialize<JObject>(reader);
            if (element is null)
                throw new JsonException($"Unable to deserialize [{objectType.FullName}]. Expecting object if not plain string.");

            if (element.TryGetValue("secure", StringComparison.OrdinalIgnoreCase, out var secureProperty) == true)
            {
                if (secureProperty.Type != JTokenType.String)
                    throw new JsonException($"Unable to deserialize [{objectType.FullName}] as a secure string. Expecting a string secret.");

                var secret = secureProperty.Value<string>();
                return new StackSettingsConfigValue(secret, true);
            }

            throw new NotSupportedException("Automation API does not currently support deserializing complex objects from stack settings.");
        }

        public override void WriteJson(
            JsonWriter writer,
            [AllowNull] StackSettingsConfigValue value,
            JsonSerializer serializer)
        {
            if (value is null)
            {
                writer.WriteNull();
            }
            // secure string
            else if (value.IsSecure)
            {
                writer.WriteStartObject();
                writer.WritePropertyName("secure");
                writer.WriteValue(value.Value);
                writer.WriteEndObject();
            }
            // plain string
            else
            {
                writer.WriteValue(value.Value);
            }
        }
    }
}
