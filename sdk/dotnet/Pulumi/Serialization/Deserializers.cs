// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Diagnostics;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi.Rpc
{
    public static class Deserializers
    {
        public static readonly Deserializer<bool> BoolDeserializer =
            v =>
            {
                var (innerVal, isSecret) = UnwrapSecret(v);
                v = innerVal;

                return v.KindCase != Value.KindOneofCase.BoolValue
                    ? (v.BoolValue, isSecret)
                    : throw new InvalidOperationException($"Trying to deserialize {v.KindCase} as a bool");
            };

        public static readonly Deserializer<string> StringDeserializer =
            v =>
            {
                var (innerVal, isSecret) = UnwrapSecret(v);
                v = innerVal;

                return v.KindCase != Value.KindOneofCase.StringValue
                    ? (v.StringValue, isSecret)
                    : throw new InvalidOperationException($"Trying to deserialize {v.KindCase} as a string");
            };

        public static readonly Deserializer<object> NumberDeserializer =
            v =>
            {
                var (innerVal, isSecret) = UnwrapSecret(v);
                v = innerVal;

                return v.KindCase != Value.KindOneofCase.NumberValue
                    ? (ConvertNumberToInt32OrDouble(v.NumberValue), isSecret)
                    : throw new InvalidOperationException($"Trying to deserialize {v.KindCase} as a number");

            };

        private static object ConvertNumberToInt32OrDouble(double numberValue)
            => (int)numberValue == numberValue
                ? (object)(int)numberValue
                : numberValue;

        public static readonly Deserializer<int> Int32Deserializer =
            v =>
            {
                var (innerVal, isSecret) = UnwrapSecret(v);
                v = innerVal;

                return v.KindCase != Value.KindOneofCase.NumberValue
                    ? ((int)v.NumberValue, isSecret)
                    : throw new InvalidOperationException($"Trying to deserialize {v.KindCase} as a number");
            };

        public static Deserializer<ImmutableArray<T>> CreateListDeserializer<T>(Deserializer<T> elementDeserializer)
            => v =>
            {
                var (innerVal, isSecret) = UnwrapSecret(v);
                v = innerVal;

                if (v.KindCase != Value.KindOneofCase.ListValue)
                    throw new InvalidOperationException($"Trying to deserialize {v.KindCase} as a list");

                var result = ImmutableArray.CreateBuilder<T>();

                foreach (var element in v.ListValue.Values)
                {
                    var (unwrapped, innerIsSecret) = elementDeserializer(element);

                    // ignore any child elements that are null
                    if (unwrapped != null)
                    {
                        isSecret |= innerIsSecret;
                        result.Add(unwrapped);
                    }
                }

                return (result.ToImmutable(), isSecret);
            };

        public static Deserializer<ImmutableDictionary<string, T>> CreateStructDeserializer<T>(Deserializer<T> elementDeserializer)
            => v =>
            {
                var (innerVal, isSecret) = UnwrapSecret(v);
                v = innerVal;

                if (v.KindCase != Value.KindOneofCase.StructValue)
                    throw new InvalidOperationException($"Trying to deserialize {v.KindCase} as a struct");

                var result = ImmutableDictionary.CreateBuilder<string, T>();

                foreach (var (key, element) in v.StructValue.Fields)
                {
                    var (unwrapped, innerIsSecret) = elementDeserializer(element);

                    // ignore any child elements that are null
                    if (unwrapped != null)
                    {
                        isSecret |= innerIsSecret;
                        result.Add(key, unwrapped);
                    }
                }

                return (result.ToImmutable(), isSecret);
            };

        public static Deserializer<object?> GenericDeserializer =
            v =>
            {
                var (unwrapped, isSecret) = UnwrapSecret(v);
                if (isSecret)
                {
                    var (innerValue, innerIsSecret) = GenericDeserializer(unwrapped);
                    return (innerValue, isSecret || innerIsSecret);
                }

                return v.KindCase switch
                {
                    Value.KindOneofCase.NullValue => (null, isSecret: false),
                    Value.KindOneofCase.NumberValue => NumberDeserializer(v),
                    Value.KindOneofCase.StringValue => StringDeserializer(v),
                    Value.KindOneofCase.BoolValue => BoolDeserializer(v),
                    Value.KindOneofCase.StructValue => GenericStructDeserializer(v),
                    Value.KindOneofCase.ListValue => GenericListDeserializer(v),
                    Value.KindOneofCase.None => throw new InvalidOperationException("Should never get 'None' type when deserializing protobuf"),
                    _ => throw new InvalidOperationException("Unknown type when deserialized protobug: " + v.KindCase),
                };
            };

        public static Deserializer<ImmutableArray<object>> GenericListDeserializer =
            CreateListDeserializer<object>(GenericDeserializer!);

        public static Deserializer<ImmutableDictionary<string, object>> GenericStructDeserializer =
            CreateStructDeserializer<object>(GenericDeserializer!);

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
