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

            // check if the element is an object,
            // if it has a single property called "secure" then it is a secret value
            // otherwise, serialize the whole object as JSON into the stack settings value
            if (element.ValueKind == JsonValueKind.Object)
            {
                foreach(var property in element.EnumerateObject())
                {
                    if (string.Equals("Secure", property.Name, StringComparison.OrdinalIgnoreCase))
                    {
                        var secureValue = property.Value;
                        if (secureValue.ValueKind != JsonValueKind.String)
                        {
                            throw new JsonException($"Unable to deserialize [{typeToConvert.FullName}] as a secure string. Expecting a string secret.");
                        }
                        
                        return new StackSettingsConfigValue(secureValue.GetString(), true);
                    }
                }
            }

            var serializedElement = JsonSerializer.Serialize(element);
            return new StackSettingsConfigValue(serializedElement, false);
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
