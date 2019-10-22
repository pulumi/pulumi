// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Diagnostics;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi.Rpc
{
    public static class Deserializers
    {
        private static Deserializer<T> CreatePrimitiveDeserializer<T>(Value.KindOneofCase kind, Func<Value, T> func)
            => CreateDeserializer(kind, v => OutputData.Create(func(v), isKnown: true, isSecret: false));

        private static Deserializer<T> CreateDeserializer<T>(Value.KindOneofCase kind, Func<Value, OutputData<T>> func)
            => CreateDeserializer(v =>
                v.KindCase == kind ? func(v) : throw new InvalidOperationException($"Trying to deserialize {v.KindCase} as a {kind}"));

        private static Deserializer<T> CreateDeserializer<T>(Func<Value, OutputData<T>> func)
            => v =>
            {
                var (innerVal, isSecret) = UnwrapSecret(v);
                v = innerVal;

                if (v.KindCase == Value.KindOneofCase.StringValue &&
                    v.StringValue == Constants.UnknownValue)
                {
                    // always deserialize unknown as the null value.
                    return new OutputData<T>(default!, isKnown: false, isSecret);
                }

                var innerData = func(v);
                return OutputData.Create(innerData.Value, innerData.IsKnown, isSecret || innerData.IsSecret);
            };

        public static readonly Deserializer<bool> BoolDeserializer =
            CreatePrimitiveDeserializer(Value.KindOneofCase.BoolValue, v => v.BoolValue);

        public static readonly Deserializer<string> StringDeserializer =
            CreatePrimitiveDeserializer(Value.KindOneofCase.StringValue, v => v.StringValue);

        public static readonly Deserializer<int> Int32Deserializer =
            CreatePrimitiveDeserializer(Value.KindOneofCase.NumberValue, v => (int)v.NumberValue);

        public static readonly Deserializer<double> DoubleDeserializer =
            CreatePrimitiveDeserializer(Value.KindOneofCase.NumberValue, v => v.NumberValue);

        public static readonly Deserializer<object> NumberDeserializer =
            CreatePrimitiveDeserializer(Value.KindOneofCase.NumberValue, v => ConvertNumberToInt32OrDouble(v.NumberValue));

        private static object ConvertNumberToInt32OrDouble(double numberValue)
            => unchecked((int)numberValue == numberValue
                ? (object)(int)numberValue
                : numberValue);

        public static Deserializer<ImmutableArray<T>> CreateListDeserializer<T>(Deserializer<T> elementDeserializer)
            => CreateDeserializer(Value.KindOneofCase.ListValue,
                v =>
                {
                    var result = ImmutableArray.CreateBuilder<T>();
                    var isKnown = true;
                    var isSecret = false;

                    foreach (var element in v.ListValue.Values)
                    {
                        var elementData = elementDeserializer(element);

                        // ignore any child elements that are null
                        if (elementData.Value != null)
                        {
                            (isKnown, isSecret) = OutputData.Combine(elementData, isKnown, isSecret);
                            result.Add(elementData.Value);
                        }
                    }

                    return OutputData.Create(result.ToImmutable(), isKnown, isSecret);
                });

        public static Deserializer<ImmutableDictionary<string, T>> CreateStructDeserializer<T>(Deserializer<T> elementDeserializer)
            => CreateDeserializer(Value.KindOneofCase.StructValue,
                v =>
                {
                    var result = ImmutableDictionary.CreateBuilder<string, T>();
                    var isKnown = true;
                    var isSecret = false;

                    foreach (var (key, element) in v.StructValue.Fields)
                    {
                        var elementData = elementDeserializer(element);

                        // ignore any child elements that are null
                        if (elementData.Value != null)
                        {
                            (isKnown, isSecret) = OutputData.Combine(elementData, isKnown, isSecret);
                            result.Add(key, elementData.Value);
                        }
                    }

                    return OutputData.Create(result.ToImmutable(), isKnown, isSecret);
                });

        public static readonly Deserializer<object> GenericDeserializer =
            CreateDeserializer(
                v => v.KindCase switch
                {
                    Value.KindOneofCase.NumberValue => NumberDeserializer(v),
                    Value.KindOneofCase.StringValue => StringDeserializer(v),
                    Value.KindOneofCase.BoolValue => BoolDeserializer(v),
                    Value.KindOneofCase.StructValue => GenericStructDeserializer(v),
                    Value.KindOneofCase.ListValue => GenericListDeserializer(v),
                    Value.KindOneofCase.NullValue => new OutputData<object?>(null, isKnown: true, isSecret: false),
                    Value.KindOneofCase.None => throw new InvalidOperationException("Should never get 'None' type when deserializing protobuf"),
                    _ => throw new InvalidOperationException("Unknown type when deserialized protobug: " + v.KindCase),
                });

        public static readonly Deserializer<ImmutableArray<object>> GenericListDeserializer =
            CreateListDeserializer(GenericDeserializer);

        public static readonly Deserializer<ImmutableDictionary<string, object>> GenericStructDeserializer =
            CreateStructDeserializer(GenericDeserializer);

        internal static (Value unwrapped, bool isSecret) UnwrapSecret(Value value)
        {
            var isSecret = false;

            while (value.KindCase == Value.KindOneofCase.StructValue &&
                   value.StructValue.Fields.TryGetValue(Constants.SpecialSigKey, out var sig) &&
                   sig.KindCase == Value.KindOneofCase.StringValue &&
                   sig.StringValue == Constants.SpecialSecretSig)
            {
                Debug.Assert(value.StructValue.Fields.TryGetValue("value", out var secretValue), "Secrets must have a field called 'value'");
                isSecret = true;
                value = secretValue;
            }

            return (value, isSecret);
        }
    }
}
