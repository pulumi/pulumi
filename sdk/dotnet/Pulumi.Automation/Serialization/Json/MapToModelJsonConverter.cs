// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Pulumi.Automation.Serialization.Json
{
    // necessary because this version of System.Text.Json
    // can't deserialize a type that doesn't have a parameterless constructor
    internal class MapToModelJsonConverter<T, TModel> : JsonConverter<T>
        where TModel : IJsonModel<T>
    {
        public override T Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
        {
            var model = JsonSerializer.Deserialize<TModel>(ref reader, options);
            if (model is null)
                throw new JsonException($"Unable to deserialize [{typeToConvert.FullName}]. Expecting object.");

            return model.Convert();
        }

        public override void Write(Utf8JsonWriter writer, T value, JsonSerializerOptions options)
        {
            JsonSerializer.Serialize(writer, value, options);
        }
    }
}
