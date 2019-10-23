// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Diagnostics;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi.Serialization
{
    internal static class Deserializer
    {
        private static OutputData<T> DeserializeCore<T>(Value value, Func<Value, OutputData<T>> func)
        {
            var (innerVal, isSecret) = UnwrapSecret(value);
            value = innerVal;

            if (value.KindCase == Value.KindOneofCase.StringValue &&
                value.StringValue == Constants.UnknownValue)
            {
                // always deserialize unknown as the null value.
                return new OutputData<T>(default!, isKnown: false, isSecret);
            }

            var innerData = func(value);
            return OutputData.Create(innerData.Value, innerData.IsKnown, isSecret || innerData.IsSecret);
        }

        private static OutputData<T> DeserializeOneOf<T>(Value value, Value.KindOneofCase kind, Func<Value, OutputData<T>> func)
            => DeserializeCore(value, v =>
                v.KindCase == kind ? func(v) : throw new InvalidOperationException($"Trying to deserialize {v.KindCase} as a {kind}"));

        private static OutputData<T> DeserializePrimitive<T>(Value value, Value.KindOneofCase kind, Func<Value, T> func)
            => DeserializeOneOf(value, kind, v => OutputData.Create(func(v), isKnown: true, isSecret: false));

        private static OutputData<bool> DeserializeBoolean(Value value)
            => DeserializePrimitive(value, Value.KindOneofCase.BoolValue, v => v.BoolValue);

        private static OutputData<string> DeserializerString(Value value)
            => DeserializePrimitive(value, Value.KindOneofCase.StringValue, v => v.StringValue);

        private static OutputData<double> DeserializerDouble(Value value)
            => DeserializePrimitive(value, Value.KindOneofCase.NumberValue, v => v.NumberValue);

        private static OutputData<ImmutableArray<T>> DeserializeList<T>(Value value, Func<Value, OutputData<T>> deserializeElement)
            => DeserializeOneOf(value, Value.KindOneofCase.ListValue,
                v =>
                {
                    var result = ImmutableArray.CreateBuilder<T>();
                    var isKnown = true;
                    var isSecret = false;

                    foreach (var element in v.ListValue.Values)
                    {
                        var elementData = deserializeElement(element);
                        (isKnown, isSecret) = OutputData.Combine(elementData, isKnown, isSecret);
                        result.Add(elementData.Value);
                    }

                    return OutputData.Create(result.ToImmutable(), isKnown, isSecret);
                });

        private static OutputData<ImmutableDictionary<string, T>> DeserializeStruct<T>(Value value, Func<Value, OutputData<T>> deserializeElement)
            => DeserializeOneOf(value, Value.KindOneofCase.StructValue,
                v =>
                {
                    var result = ImmutableDictionary.CreateBuilder<string, T>();
                    var isKnown = true;
                    var isSecret = false;

                    foreach (var (key, element) in v.StructValue.Fields)
                    {
                        var elementData = deserializeElement(element);
                        (isKnown, isSecret) = OutputData.Combine(elementData, isKnown, isSecret);
                        result.Add(key, elementData.Value);
                    }

                    return OutputData.Create(result.ToImmutable(), isKnown, isSecret);
                });

        public static OutputData<object?> Deserialize(Value value)
            => DeserializeCore(value, 
                v => v.KindCase switch
                {
                    Value.KindOneofCase.NumberValue => DeserializerDouble(v),
                    Value.KindOneofCase.StringValue => DeserializerString(v),
                    Value.KindOneofCase.BoolValue => DeserializeBoolean(v),
                    Value.KindOneofCase.StructValue => DeserializerStruct(v),
                    Value.KindOneofCase.ListValue => DeserializeList(v),
                    Value.KindOneofCase.NullValue => new OutputData<object?>(null, isKnown: true, isSecret: false),
                    Value.KindOneofCase.None => throw new InvalidOperationException("Should never get 'None' type when deserializing protobuf"),
                    _ => throw new InvalidOperationException("Unknown type when deserialized protobug: " + v.KindCase),
                });

        private static OutputData<ImmutableArray<object?>> DeserializeList(Value value)
            => DeserializeList(value, v => Deserialize(v));

        public static OutputData<ImmutableDictionary<string, object>> DeserializerStruct(Value value)
            => DeserializeStruct(value, v => Deserialize(v))!;

        internal static (Value unwrapped, bool isSecret) UnwrapSecret(Value value)
        {
            var isSecret = false;

            while (value.KindCase == Value.KindOneofCase.StructValue &&
                   value.StructValue.Fields.TryGetValue(Constants.SpecialSigKey, out var sig) &&
                   sig.KindCase == Value.KindOneofCase.StringValue &&
                   sig.StringValue == Constants.SpecialSecretSig)
            {
                Debug.Assert(value.StructValue.Fields.TryGetValue(Constants.SecretValueName, out var secretValue), "Secrets must have a field called 'value'");
                isSecret = true;
                value = secretValue;
            }

            return (value, isSecret);
        }
    }
}
