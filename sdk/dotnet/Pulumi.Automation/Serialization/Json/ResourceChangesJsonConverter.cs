using System;
using System.Collections.Generic;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Pulumi.Automation.Serialization.Json
{
    internal class ResourceChangesJsonConverter : JsonConverter<Dictionary<OperationType, int>>
    {
        public override Dictionary<OperationType, int> Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
        {
            if (reader.TokenType != JsonTokenType.StartObject)
                throw new JsonException($"Cannot deserialize [{typeToConvert.FullName}]. Expecing object.");

            var dictionary = new Dictionary<OperationType, int>();

            reader.Read();
            while (reader.TokenType != JsonTokenType.EndObject)
            {
                if (reader.TokenType != JsonTokenType.PropertyName)
                    throw new JsonException("Expecting property name.");

                var propertyName = reader.GetString();
                if (string.IsNullOrWhiteSpace(propertyName))
                    throw new JsonException("Unable to retrieve property name.");

                var operationType = ConvertToOperationType(propertyName);

                reader.Read();
                if (reader.TokenType != JsonTokenType.Number
                    || !reader.TryGetInt32(out var count))
                    throw new JsonException("Expecting number.");

                dictionary[operationType] = count;
                reader.Read();
            }

            return dictionary;
        }

        public override void Write(Utf8JsonWriter writer, Dictionary<OperationType, int> value, JsonSerializerOptions options)
        {
            throw new NotImplementedException();
        }

        private static OperationType ConvertToOperationType(string opType)
        {
            switch (opType)
            {
                case "create":
                    return OperationType.Create;
                case "create-replacement":
                    return OperationType.CreateReplacement;
                case "delete":
                    return OperationType.Delete;
                case "delete-replaced":
                    return OperationType.DeleteReplaced;
                case "replace":
                    return OperationType.Replace;
                case "same":
                    return OperationType.Same;
                case "update":
                    return OperationType.Update;
                default:
                    throw new JsonException($"Invalid operation type: {opType}");
            }
        }
    }
}
