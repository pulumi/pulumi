// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Pulumi.Automation.Serialization.Json
{
    internal class StringToEnumJsonConverter<TEnum, TConverter> : JsonConverter<TEnum>
        where TEnum : struct, Enum
        where TConverter : IStringToEnumConverter<TEnum>, new()
    {
        private readonly TConverter _converter = new TConverter();

        public override TEnum Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
        {
            return _converter.Convert(reader.GetString());
        }

        public override void Write(Utf8JsonWriter writer, TEnum value, JsonSerializerOptions options)
        {
            throw new NotImplementedException();
        }
    }
}
