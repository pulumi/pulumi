using System;
using System.Diagnostics.CodeAnalysis;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;

namespace Pulumi.X.Automation.Serialization.Json
{
    internal class ProjectRuntimeJsonConverter : JsonConverter<ProjectRuntime>
    {
        public override ProjectRuntime ReadJson(
            JsonReader reader,
            Type objectType,
            [AllowNull] ProjectRuntime existingValue,
            bool hasExistingValue,
            JsonSerializer serializer)
        {
            if (reader.TokenType is JsonToken.String)
            {
                var runtimeName = DeserializeName(reader.Value as string, objectType);
                return new ProjectRuntime(runtimeName);
            }

            if (reader.TokenType != JsonToken.StartObject)
                throw new JsonException($"Unable to deserialize [{objectType.FullName}]. Expecting string or object.");

            var element = serializer.Deserialize<JObject>(reader);
            if (element is null)
                throw new JsonException($"Unable to deserialize [{objectType.FullName}]. Expecting string or object.");

            if (!element.TryGetValue(nameof(ProjectRuntime.Name), StringComparison.OrdinalIgnoreCase, out var nameProperty))
                throw new JsonException($"Unable to deserialize [{objectType.FullName}]. Expecting runtime name property.");

            if (nameProperty.Type != JTokenType.String)
                throw new JsonException($"Unable to deserialize [{objectType.FullName}]. Runtime name property should be a string.");

            var name = DeserializeName(nameProperty.Value<string>(), objectType);

            if (!element.TryGetValue(nameof(ProjectRuntime.Options), StringComparison.OrdinalIgnoreCase, out var optionsProperty))
                return new ProjectRuntime(name);

            if (optionsProperty.Type != JTokenType.Object)
                throw new JsonException($"Unable to deserialize [{objectType.FullName}]. Runtime options property should be an object.");

            var runtimeOptions = serializer.Deserialize<ProjectRuntimeOptions>(optionsProperty.CreateReader());
            return new ProjectRuntime(name) { Options = runtimeOptions };

            static ProjectRuntimeName DeserializeName(string? stringValue, Type objectType)
            {
                if (string.IsNullOrWhiteSpace(stringValue))
                    throw new JsonException($"A valid runtime name was not provided when deserializing [{objectType.FullName}].");

                if (Enum.TryParse<ProjectRuntimeName>(stringValue, true, out var runtimeName))
                    return runtimeName;

                throw new JsonException($"Unexpected runtime name of \"{stringValue}\" provided when deserializing [{objectType.FullName}].");
            }
        }

        public override void WriteJson(
            JsonWriter writer,
            [AllowNull] ProjectRuntime value,
            JsonSerializer serializer)
        {
            if (value is null)
            {
                writer.WriteNull();
            }
            else if (value.Options is null)
            {
                writer.WriteValue(value.Name.ToString().ToLower());
            }
            else
            {
                writer.WriteStartObject();
                writer.WritePropertyName("name");
                writer.WriteValue(value.Name.ToString().ToLower());
                writer.WritePropertyName("options");
                serializer.Serialize(writer, value.Options);
                writer.WriteEndObject();
            }
        }
    }
}
