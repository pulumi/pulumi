// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections;
using System.Collections.Immutable;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using OneOf;

namespace Pulumi.Serialization
{
    internal struct Serializer
    {
        public readonly HashSet<Resource> DependentResources;

        private readonly bool _excessiveDebugOutput;

        public Serializer(bool excessiveDebugOutput)
        {
            this.DependentResources = new HashSet<Resource>();
            _excessiveDebugOutput = excessiveDebugOutput;
        }

        /// <summary>
        /// Takes in an arbitrary object and serializes it into a uniform form that can converted
        /// trivially to a protobuf to be passed to the Pulumi engine.
        /// <para/>
        /// The allowed 'basis' forms that can be serialized are:
        /// <list type="number">
        /// <item><see langword="null"/>s</item>
        /// <item><see cref="bool"/>s</item>
        /// <item><see cref="int"/>s</item>
        /// <item><see cref="double"/>s</item>
        /// <item><see cref="string"/>s</item>
        /// <item><see cref="Asset"/>s</item>
        /// <item><see cref="Archive"/>s</item>
        /// <item><see cref="Resource"/>s</item>
        /// <item><see cref="ResourceArgs"/>s</item>
        /// </list>
        /// Additionally, other more complex objects can be serialized as long as they are built
        /// out of serializable objects.  These complex objects include:
        /// <list type="number">
        /// <item><see cref="Input{T}"/>s. As long as they are an Input of a serializable type.</item>
        /// <item><see cref="Output{T}"/>s. As long as they are an Output of a serializable type.</item>
        /// <item><see cref="IList"/>s. As long as all elements in the list are serializable.</item>
        /// <item><see cref="IDictionary"/>. As long as the key of the dictionary are <see cref="string"/>s and as long as the value are all serializable.</item>
        /// </list>
        /// No other forms are allowed.
        /// <para/>
        /// This function will only return values of a very specific shape.  Specifically, the
        /// result values returned will *only* be one of:
        /// <para/>
        /// <list type="number">
        /// <item><see langword="null"/></item>
        /// <item><see cref="bool"/></item>
        /// <item><see cref="int"/></item>
        /// <item><see cref="double"/></item>
        /// <item><see cref="string"/></item>
        /// <item>An <see cref="ImmutableArray{T}"/> containing only these result value types.</item>
        /// <item>An <see cref="IImmutableDictionary{TKey, TValue}"/> where the keys are strings and
        /// the values are only these result value types.</item>
        /// </list>
        /// No other result type are allowed to be returned.
        /// </summary>
        public async Task<object?> SerializeAsync(string ctx, object? prop)
        {
            // IMPORTANT:
            // IMPORTANT: Keep this in sync with serializesPropertiesSync in invoke.ts
            // IMPORTANT:
            if (prop == null ||
                prop is bool ||
                prop is int ||
                prop is double ||
                prop is string)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: primitive={prop}");
                }

                return prop;
            }

            if (prop is ResourceArgs args)
                return await SerializeResourceArgsAsync(ctx, args).ConfigureAwait(false);

            if (prop is AssetOrArchive assetOrArchive)
                return await SerializeAssetOrArchiveAsync(ctx, assetOrArchive).ConfigureAwait(false);

            if (prop is Task)
            {
                throw new InvalidOperationException(
$"Tasks are not allowed inside ResourceArgs. Please wrap your Task in an Output:\n\t{ctx}");
            }

            if (prop is IInput input)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Recursing into IInput");
                }

                return await SerializeAsync(ctx, input.ToOutput()).ConfigureAwait(false);
            }

            if (prop is IOneOf oneOf)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Recursing into IOneOf");
                }

                return await SerializeAsync(ctx, oneOf.Value).ConfigureAwait(false);
            }

            if (prop is IOutput output)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Recursing into Output");
                }

                this.DependentResources.AddRange(output.Resources);
                var data = await output.GetDataAsync().ConfigureAwait(false);

                // When serializing an Output, we will either serialize it as its resolved value or the "unknown value"
                // sentinel. We will do the former for all outputs created directly by user code (such outputs always
                // resolve isKnown to true) and for any resource outputs that were resolved with known values.
                var isKnown = data.IsKnown;
                var isSecret = data.IsSecret;

                if (!isKnown)
                    return Constants.UnknownValue;

                var value = await SerializeAsync($"{ctx}.id", data.Value).ConfigureAwait(false);
                if (isSecret)
                {
                    var builder = ImmutableDictionary.CreateBuilder<string, object?>();
                    builder.Add(Constants.SpecialSigKey, Constants.SpecialSecretSig);
                    builder.Add(Constants.SecretValueName, value);
                    return builder.ToImmutable();
                }

                return value;
            }

            if (prop is CustomResource customResource)
            {
                // Resources aren't serializable; instead, we serialize them as references to the ID property.
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Encountered CustomResource");
                }

                this.DependentResources.Add(customResource);
                return await SerializeAsync($"{ctx}.id", customResource.Id).ConfigureAwait(false);
            }

            if (prop is ComponentResource componentResource)
            {
                // Component resources often can contain cycles in them.  For example, an awsinfra
                // SecurityGroupRule can point a the awsinfra SecurityGroup, which in turn can point
                // back to its rules through its 'egressRules' and 'ingressRules' properties.  If
                // serializing out the 'SecurityGroup' resource ends up trying to serialize out
                // those properties, a deadlock will happen, due to waiting on the child, which is
                // waiting on the parent.
                //
                // Practically, there is no need to actually serialize out a component.  It doesn't
                // represent a real resource, nor does it have normal properties that need to be
                // tracked for differences (since changes to its properties don't represent changes
                // to resources in the real world).
                //
                // So, to avoid these problems, while allowing a flexible and simple programming
                // model, we just serialize out the component as its urn.  This allows the component
                // to be identified and tracked in a reasonable manner, while not causing us to
                // compute or embed information about it that is not needed, and which can lead to
                // deadlocks.
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Encountered ComponentResource");
                }

                return await SerializeAsync($"{ctx}.urn", componentResource.Urn).ConfigureAwait(false);
            }

            if (prop is IDictionary dictionary)
                return await SerializeDictionaryAsync(ctx, dictionary).ConfigureAwait(false);

            if (prop is IList list)
                return await SerializeListAsync(ctx, list).ConfigureAwait(false);

            throw new InvalidOperationException($"{prop.GetType().FullName} is not a supported argument type.\n\t{ctx}");
        }

        private async Task<ImmutableDictionary<string, object>> SerializeAssetOrArchiveAsync(string ctx, AssetOrArchive assetOrArchive)
        {
            if (_excessiveDebugOutput)
            {
                Log.Debug($"Serialize property[{ctx}]: asset/archive={assetOrArchive.GetType().Name}");
            }

            var propName = assetOrArchive.PropName;
            var value = await SerializeAsync(ctx + "." + propName, assetOrArchive.Value).ConfigureAwait(false);

            var builder = ImmutableDictionary.CreateBuilder<string, object>();
            builder.Add(Constants.SpecialSigKey, assetOrArchive.SigKey);
            builder.Add(assetOrArchive.PropName, value!);
            return builder.ToImmutable();
        }

        private async Task<ImmutableDictionary<string, object>> SerializeResourceArgsAsync(string ctx, ResourceArgs args)
        {
            if (_excessiveDebugOutput)
            {
                Log.Debug($"Serialize property[{ctx}]: Recursing into ResourceArgs");
            }

            var dictionary = await args.ToDictionaryAsync().ConfigureAwait(false);
            return await SerializeDictionaryAsync(ctx, dictionary).ConfigureAwait(false);
        }

        private async Task<ImmutableArray<object?>> SerializeListAsync(string ctx, IList list)
        {
            if (_excessiveDebugOutput)
            {
                Log.Debug($"Serialize property[{ctx}]: Hit list");
            }

            var result = ImmutableArray.CreateBuilder<object?>(list.Count);
            for (int i = 0, n = list.Count; i < n; i++)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: array[{i}] element");
                }

                result.Add(await SerializeAsync($"{ctx}[{i}]", list[i]).ConfigureAwait(false));
            }

            return result.MoveToImmutable();
        }

        private async Task<ImmutableDictionary<string, object>> SerializeDictionaryAsync(string ctx, IDictionary dictionary)
        {
            if (_excessiveDebugOutput)
            {
                Log.Debug($"Serialize property[{ctx}]: Hit dictionary");
            }

            var result = ImmutableDictionary.CreateBuilder<string, object>();
            foreach (var key in dictionary.Keys)
            {
                if (!(key is string stringKey))
                {
                    throw new InvalidOperationException(
                        $"Dictionaries are only supported with string keys:\n\t{ctx}");
                }

                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: object.{stringKey}");
                }

                // When serializing an object, we omit any keys with null values. This matches
                // JSON semantics.
                var v = await SerializeAsync($"{ctx}.{stringKey}", dictionary[stringKey]).ConfigureAwait(false);
                if (v != null)
                {
                    result[stringKey] = v;
                }
            }

            return result.ToImmutable();
        }

        /// <summary>
        /// Internal for testing purposes.
        /// </summary>
        internal static Value CreateValue(object? value)
            => value switch
            {
                null => Value.ForNull(),
                int i => Value.ForNumber(i),
                double d => Value.ForNumber(d),
                bool b => Value.ForBool(b),
                string s => Value.ForString(s),
                ImmutableArray<object> list => Value.ForList(list.Select(v => CreateValue(v)).ToArray()),
                ImmutableDictionary<string, object> dict => Value.ForStruct(CreateStruct(dict)),
                _ => throw new InvalidOperationException("Unsupported value when converting to protobuf: " + value.GetType().FullName),
            };

        /// <summary>
        /// Given a <see cref="ImmutableDictionary{TKey, TValue}"/> produced by <see cref="SerializeAsync"/>,
        /// produces the equivalent <see cref="Struct"/> that can be passed to the Pulumi engine.
        /// </summary>
        public static Struct CreateStruct(ImmutableDictionary<string, object> serializedDictionary)
        {
            var result = new Struct();
            foreach (var key in serializedDictionary.Keys.OrderBy(k => k))
            {
                result.Fields.Add(key, CreateValue(serializedDictionary[key]));
            }
            return result;
        }
    }
}
