// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Pulumi.Automation.Serialization.Json
{
    internal class ProjectRuntimeJsonConverter : JsonConverter<ProjectRuntime>
    {
        public override ProjectRuntime Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
        {
            if (reader.TokenType == JsonTokenType.String)
            {
                var runtimeName = DeserializeName(ref reader, typeToConvert);
                return new ProjectRuntime(runtimeName);
            }

            if (reader.TokenType != JsonTokenType.StartObject)
                throw new JsonException($"Unable to deserialize [{typeToConvert.FullName}]. Expecting string or object.");

            reader.Read();
            if (reader.TokenType != JsonTokenType.PropertyName
                || !string.Equals(nameof(ProjectRuntime.Name), reader.GetString(), StringComparison.OrdinalIgnoreCase))
                throw new JsonException($"Unable to deserialize [{typeToConvert.FullName}]. Expecting runtime name property.");

            reader.Read();
            if (reader.TokenType != JsonTokenType.String)
                throw new JsonException($"Unable to deserialize [{typeToConvert.FullName}]. Runtime name property should be a string.");

            var name = DeserializeName(ref reader, typeToConvert);

            reader.Read();
            if (reader.TokenType == JsonTokenType.EndObject)
                return new ProjectRuntime(name);

            if (reader.TokenType != JsonTokenType.PropertyName
                || !string.Equals(nameof(ProjectRuntime.Options), reader.GetString(), StringComparison.OrdinalIgnoreCase))
                throw new JsonException($"Unable to deserialize [{typeToConvert.FullName}]. Expecting runtime options property.");

            reader.Read();
            if (reader.TokenType != JsonTokenType.StartObject)
                throw new JsonException($"Unable to deserialize [{typeToConvert.FullName}]. Runtime options property should be an object.");

            var runtimeOptions = JsonSerializer.Deserialize<ProjectRuntimeOptions>(ref reader, options);
            reader.Read(); // read final EndObject token

            return new ProjectRuntime(name) { Options = runtimeOptions };

            static ProjectRuntimeName DeserializeName(ref Utf8JsonReader reader, Type typeToConvert)
            {
                var runtimeStr = reader.GetString();
                if (string.IsNullOrWhiteSpace(runtimeStr))
                    throw new JsonException($"A valid runtime name was not provided when deserializing [{typeToConvert.FullName}].");

                if (Enum.TryParse<ProjectRuntimeName>(runtimeStr, true, out var runtimeName))
                    return runtimeName;

                throw new JsonException($"Unexpected runtime name of \"{runtimeStr}\" provided when deserializing [{typeToConvert.FullName}].");
            }
        }

        public override void Write(Utf8JsonWriter writer, ProjectRuntime value, JsonSerializerOptions options)
        {
            if (value.Options is null)
            {
                writer.WriteStringValue(value.Name.ToString().ToLower());
            }
            else
            {
                JsonSerializer.Serialize(writer, value);
            }
        }
    }
}
