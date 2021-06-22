// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Pulumi.Automation.Serialization.Json
{
    internal class SystemObjectJsonConverter : JsonConverter<object>
    {
        public override object Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
        {
            if (reader.TokenType == JsonTokenType.True)
            {
                return true;
            }

            if (reader.TokenType == JsonTokenType.False)
            {
                return false;
            }

            if (reader.TokenType == JsonTokenType.Number)
            {
                if (reader.TryGetInt64(out var l))
                {
                    return l;
                }

                return reader.GetDouble();
            }

            if (reader.TokenType == JsonTokenType.String)
            {
                if (reader.TryGetDateTime(out var datetime))
                {
                    return datetime;
                }

                return reader.GetString();
            }

            if (reader.TokenType == JsonTokenType.StartArray)
            {
                return JsonSerializer.Deserialize<object[]>(ref reader, options);
            }

            if (reader.TokenType == JsonTokenType.StartObject)
            {
                var dictionary = new Dictionary<string, object>();

                reader.Read();
                while (reader.TokenType != JsonTokenType.EndObject)
                {
                    if (reader.TokenType != JsonTokenType.PropertyName)
                        throw new JsonException("Expecting property name.");

                    var propertyName = reader.GetString();
                    if (string.IsNullOrWhiteSpace(propertyName))
                        throw new JsonException("Unable to retrieve property name.");

                    reader.Read();
                    dictionary[propertyName] = JsonSerializer.Deserialize<object>(ref reader, options);

                    reader.Read();
                }

                return dictionary;
            }

            throw new JsonException("Invalid JSON element.");
        }

        public override void Write(Utf8JsonWriter writer, object value, JsonSerializerOptions options)
        {
            throw new NotSupportedException($"Writing as [{typeof(object).FullName}] is not supported.");
        }
    }
}
