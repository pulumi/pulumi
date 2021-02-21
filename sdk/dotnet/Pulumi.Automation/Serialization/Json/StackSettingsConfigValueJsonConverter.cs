// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Pulumi.Automation.Serialization.Json
{
    internal class StackSettingsConfigValueJsonConverter : JsonConverter<StackSettingsConfigValue>
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

            throw new NotSupportedException("Automation API does not currently support deserializing complex objects from stack settings.");
        }

        public override void Write(Utf8JsonWriter writer, StackSettingsConfigValue value, JsonSerializerOptions options)
        {
            // secure string
            if (value.IsSecure)
            {
                var securePropertyName = options.PropertyNamingPolicy?.ConvertName("Secure") ?? "Secure";
                writer.WriteStartObject();
                writer.WriteString(securePropertyName, value.Value);
                writer.WriteEndObject();
            }
            // plain string
            else
            {
                writer.WriteStringValue(value.Value);
            }
        }
    }
}
