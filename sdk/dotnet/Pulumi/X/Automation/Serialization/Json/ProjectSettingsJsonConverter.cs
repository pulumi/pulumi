using System;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Pulumi.X.Automation.Serialization.Json
{
    // necessary because this version of System.Text.Json
    // can't deserialize a type that doesn't have a parameterless constructor
    internal class ProjectSettingsJsonConverter : JsonConverter<ProjectSettings>
    {
        public override ProjectSettings Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
        {
            var model = JsonSerializer.Deserialize<ProjectSettingsModel>(ref reader, options);
            if (model is null)
                throw new JsonException($"Unable to deserialize [{typeToConvert.FullName}]. Expecting object.");

            return model.Convert();
        }

        public override void Write(Utf8JsonWriter writer, ProjectSettings value, JsonSerializerOptions options)
        {
            JsonSerializer.Serialize(writer, value, options);
        }
    }
}
