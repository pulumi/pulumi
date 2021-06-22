// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Immutable;
using System.Diagnostics.CodeAnalysis;
using Google.Protobuf.Collections;
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
                return new OutputData<T>(ImmutableHashSet<Resource>.Empty, default!, isKnown: false, isSecret);
            }

            if (TryDeserializeAssetOrArchive(value, out var assetOrArchive))
            {
                return new OutputData<T>(ImmutableHashSet<Resource>.Empty, (T)(object)assetOrArchive, isKnown: true, isSecret);
            }
            if (TryDeserializeResource(value, out var resource))
            {
                return new OutputData<T>(ImmutableHashSet<Resource>.Empty, (T)(object)resource, isKnown: true, isSecret);
            }

            var innerData = func(value);
            return OutputData.Create(innerData.Resources, innerData.Value, innerData.IsKnown, isSecret || innerData.IsSecret);
        }

        private static OutputData<T> DeserializeOneOf<T>(Value value, Value.KindOneofCase kind, Func<Value, OutputData<T>> func)
            => DeserializeCore(value, v =>
                v.KindCase == kind ? func(v) : throw new InvalidOperationException($"Trying to deserialize {v.KindCase} as a {kind}"));

        private static OutputData<T> DeserializePrimitive<T>(Value value, Value.KindOneofCase kind, Func<Value, T> func)
            => DeserializeOneOf(value, kind, v => OutputData.Create(
                ImmutableHashSet<Resource>.Empty, func(v), isKnown: true, isSecret: false));

        private static OutputData<bool> DeserializeBoolean(Value value)
            => DeserializePrimitive(value, Value.KindOneofCase.BoolValue, v => v.BoolValue);

        private static OutputData<string> DeserializerString(Value value)
            => DeserializePrimitive(value, Value.KindOneofCase.StringValue, v => v.StringValue);

        private static OutputData<double> DeserializerDouble(Value value)
            => DeserializePrimitive(value, Value.KindOneofCase.NumberValue, v => v.NumberValue);

        private static OutputData<ImmutableArray<object?>> DeserializeList(Value value)
            => DeserializeOneOf(value, Value.KindOneofCase.ListValue,
                v =>
                {
                    var resources = ImmutableHashSet.CreateBuilder<Resource>();
                    var result = ImmutableArray.CreateBuilder<object?>();
                    var isKnown = true;
                    var isSecret = false;

                    foreach (var element in v.ListValue.Values)
                    {
                        var elementData = Deserialize(element);
                        (isKnown, isSecret) = OutputData.Combine(elementData, isKnown, isSecret);
                        resources.UnionWith(elementData.Resources);
                        result.Add(elementData.Value);
                    }

                    return OutputData.Create(resources.ToImmutable(), result.ToImmutable(), isKnown, isSecret);
                });

        private static OutputData<ImmutableDictionary<string, object?>> DeserializeStruct(Value value)
            => DeserializeOneOf(value, Value.KindOneofCase.StructValue,
                v =>
                {
                    var resources = ImmutableHashSet.CreateBuilder<Resource>();
                    var result = ImmutableDictionary.CreateBuilder<string, object?>();
                    var isKnown = true;
                    var isSecret = false;

                    foreach (var (key, element) in v.StructValue.Fields)
                    {
                        // Unilaterally skip properties considered internal by the Pulumi engine.
                        // These don't actually contribute to the exposed shape of the object, do
                        // not need to be passed back to the engine, and often will not match the
                        // expected type we are deserializing into.
                        if (key.StartsWith("__", StringComparison.Ordinal))
                            continue;

                        var elementData = Deserialize(element);
                        (isKnown, isSecret) = OutputData.Combine(elementData, isKnown, isSecret);
                        result.Add(key, elementData.Value);
                        resources.UnionWith(elementData.Resources);
                    }

                    return OutputData.Create(resources.ToImmutable(), result.ToImmutable(), isKnown, isSecret);
                });

        public static OutputData<object?> Deserialize(Value value)
            => DeserializeCore(value, 
                v => v.KindCase switch
                {
                    Value.KindOneofCase.NumberValue => DeserializerDouble(v),
                    Value.KindOneofCase.StringValue => DeserializerString(v),
                    Value.KindOneofCase.BoolValue => DeserializeBoolean(v),
                    Value.KindOneofCase.StructValue => DeserializeStruct(v),
                    Value.KindOneofCase.ListValue => DeserializeList(v),
                    Value.KindOneofCase.NullValue => new OutputData<object?>(ImmutableHashSet<Resource>.Empty, null, isKnown: true, isSecret: false),
                    Value.KindOneofCase.None => throw new InvalidOperationException("Should never get 'None' type when deserializing protobuf"),
                    _ => throw new InvalidOperationException("Unknown type when deserializing protobuf: " + v.KindCase),
                });

        private static (Value unwrapped, bool isSecret) UnwrapSecret(Value value)
        {
            var isSecret = false;

            while (IsSpecialStruct(value, out var sig) &&
                   sig == Constants.SpecialSecretSig)
            {
                if (!value.StructValue.Fields.TryGetValue(Constants.SecretValueName, out var secretValue))
                    throw new InvalidOperationException("Secrets must have a field called 'value'");

                isSecret = true;
                value = secretValue;
            }

            return (value, isSecret);
        }

        private static bool IsSpecialStruct(
            Value value, [NotNullWhen(true)] out string? sig)
        {
            if (value.KindCase == Value.KindOneofCase.StructValue &&
                value.StructValue.Fields.TryGetValue(Constants.SpecialSigKey, out var sigVal) &&
                sigVal.KindCase == Value.KindOneofCase.StringValue)
            {
                sig = sigVal.StringValue;
                return true;
            }

            sig = null;
            return false;
        }

        private static bool TryDeserializeAssetOrArchive(
            Value value, [NotNullWhen(true)] out AssetOrArchive? assetOrArchive)
        {
            if (IsSpecialStruct(value, out var sig))
            {
                if (sig == Constants.SpecialAssetSig)
                {
                    assetOrArchive = DeserializeAsset(value);
                    return true;
                }
                if (sig == Constants.SpecialArchiveSig)
                {
                    assetOrArchive = DeserializeArchive(value);
                    return true;
                }
            }

            assetOrArchive = null;
            return false;
        }

        private static Archive DeserializeArchive(Value value)
        {
            if (TryGetStringValue(value.StructValue.Fields, Constants.AssetOrArchivePathName, out var path))
                return new FileArchive(path);

            if (TryGetStringValue(value.StructValue.Fields, Constants.AssetOrArchiveUriName, out var uri))
                return new RemoteArchive(uri);

            if (value.StructValue.Fields.TryGetValue(Constants.ArchiveAssetsName, out var assetsValue))
            {
                if (assetsValue.KindCase == Value.KindOneofCase.StructValue)
                {
                    var assets = ImmutableDictionary.CreateBuilder<string, AssetOrArchive>();
                    foreach (var (name, val) in assetsValue.StructValue.Fields)
                    {
                        if (!TryDeserializeAssetOrArchive(val, out var innerAssetOrArchive))
                            throw new InvalidOperationException("AssetArchive contained an element that wasn't itself an Asset or Archive.");

                        assets[name] = innerAssetOrArchive;
                    }

                    return new AssetArchive(assets.ToImmutable());
                }
            }

            throw new InvalidOperationException("Value was marked as Archive, but did not conform to required shape.");
        }

        private static Asset DeserializeAsset(Value value)
        {
            if (TryGetStringValue(value.StructValue.Fields, Constants.AssetOrArchivePathName, out var path))
                return new FileAsset(path);

            if (TryGetStringValue(value.StructValue.Fields, Constants.AssetOrArchiveUriName, out var uri))
                return new RemoteAsset(uri);

            if (TryGetStringValue(value.StructValue.Fields, Constants.AssetTextName, out var text))
                return new StringAsset(text);

            throw new InvalidOperationException("Value was marked as Asset, but did not conform to required shape.");
        }

        private static bool TryDeserializeResource(
            Value value, [NotNullWhen(true)] out Resource? resource)
        {
            if (!IsSpecialStruct(value, out var sig) || sig != Constants.SpecialResourceSig)
            {
                resource = null;
                return false;
            }

            if (!TryGetStringValue(value.StructValue.Fields, Constants.ResourceUrnName, out var urn))
            {
                throw new InvalidOperationException("Value was marked as a Resource, but did not conform to required shape.");
            }

            if (!TryGetStringValue(value.StructValue.Fields, Constants.ResourceVersionName, out var version)) {
                version = "";
            }

            var urnParts = urn.Split("::");
            var qualifiedType = urnParts[2];
            var qualifiedTypeParts = qualifiedType.Split('$');
            var type = qualifiedTypeParts[^1];

            if (ResourcePackages.TryConstruct(type, version, urn, out resource)) {
                return true;
            }

            resource = new DependencyResource(urn);
            return true;
        }

        private static bool TryGetStringValue(
            MapField<string, Value> fields, string keyName, [NotNullWhen(true)] out string? result)
        {
            if (fields.TryGetValue(keyName, out var value) &&
                value.KindCase == Value.KindOneofCase.StringValue)
            {
                result = value.StringValue;
                return true;
            }

            result = null;
            return false;
        }
    }
}
