using System;
using System.Collections.Generic;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Pulumi.X.Automation.Serialization.Json
{
    internal class StackSettingsConfigValueConverter : JsonConverter<StackSettingsConfigValue>
    {
        public override StackSettingsConfigValue Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
        {
            var element = JsonSerializer.Deserialize<JsonElement>(ref reader, options);
            
            // check if plain string
            if (element.ValueKind == JsonValueKind.String)
            {
                var value = element.GetString();
                return new StackSettingsConfigValue(value, false);
            }

            // confirm object
            if (element.ValueKind != JsonValueKind.Object)
                throw new JsonException($"Unable to deserialize [{typeToConvert.FullName}]. Expecting object if not plain string.");

            // check if secure string
            var securePropertyName = options.PropertyNamingPolicy?.ConvertName("Secure") ?? "Secure";
            if (element.TryGetProperty(securePropertyName, out var secureProperty))
            {
                if (secureProperty.ValueKind != JsonValueKind.String)
                    throw new JsonException($"Unable to deserialize [{typeToConvert.FullName}] as a secure string. Expecting a string secret.");

                var secret = secureProperty.GetString();
                return new StackSettingsConfigValue(secret, true);
            }

            var dictionary = JsonSerializer.Deserialize<IDictionary<string, object>>(element.GetRawText(), options);
            return new StackSettingsConfigValue(dictionary);
        }

        public override void Write(Utf8JsonWriter writer, StackSettingsConfigValue value, JsonSerializerOptions options)
        {
            // object
            if (value.IsObject)
            {
                JsonSerializer.Serialize(writer, value.ValueObject, options: options);
            }
            // secure string
            else if (value.IsSecure)
            {
                var securePropertyName = options.PropertyNamingPolicy?.ConvertName("Secure") ?? "Secure";
                writer.WriteStartObject();
                writer.WriteString(securePropertyName, value.ValueString);
                writer.WriteEndObject();
            }
            // plain string
            else
            {
                writer.WriteStringValue(value.ValueString);
            }
        }
    }
}
