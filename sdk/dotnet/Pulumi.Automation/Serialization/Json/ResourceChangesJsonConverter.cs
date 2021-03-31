// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Pulumi.Automation.Serialization.Json
{
    internal class ResourceChangesJsonConverter : JsonConverter<Dictionary<OperationType, int>>
    {
        private readonly OperationTypeConverter _operationTypeConverter = new OperationTypeConverter();

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

                var operationType = _operationTypeConverter.Convert(propertyName);

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
    }
}
