using System;
using System.Diagnostics.CodeAnalysis;
using Newtonsoft.Json;

namespace Pulumi.X.Automation.Serialization.Json
{
    // necessary for constructor deserialization
    internal class MapToModelJsonConverter<T, TModel> : JsonConverter<T>
        where TModel : IJsonModel<T>
    {
        public override T ReadJson(
            JsonReader reader,
            Type objectType,
            [AllowNull] T existingValue,
            bool hasExistingValue,
            JsonSerializer serializer)
        {
            var model = serializer.Deserialize<TModel>(reader);
            if (model is null)
                throw new JsonException($"Unable to deserialize [{objectType.FullName}]. Expecting object.");

            return model.Convert();
        }

        public override bool CanWrite => false;

        public override void WriteJson(
            JsonWriter writer,
            [AllowNull] T value,
            JsonSerializer serializer)
        {
            throw new NotImplementedException();
        }
    }
}
