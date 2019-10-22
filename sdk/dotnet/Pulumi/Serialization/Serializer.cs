// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Generic;
using System.Threading.Tasks;

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

        public Task<object?> SerializeAsync(object? prop)
            => SerializeAsync(ctx: "", prop);

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
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Recursing into ResourceArgs");
                }

                return await SerializeAsync(ctx, args.ToDictionary()).ConfigureAwait(false);
            }

            if (prop is AssetOrArchive assetOrArchive)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: asset/archive={assetOrArchive.GetType().Name}");
                }

                var (sig, propName, propValue) = assetOrArchive.GetSerializationData();
                var result = new Dictionary<string, object?>
                {
                    { Constants.SpecialSigKey, sig },
                };

                result[propName] = await SerializeAsync(
                    ctx + "." + propName, propValue).ConfigureAwait(false);
                return result;
            }

            if (prop is Task)
            {
                throw new InvalidOperationException(
$"Tasks are not allowed inside ResourceArgs. Please wrap your Task in an Output:\n\t{ctx}");
            }

            if (prop is IInput input)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Recursing into input");
                }

                return await SerializeAsync(ctx, input.ToOutput()).ConfigureAwait(false);
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
                var value = await SerializeAsync($"{ctx}.id", data.Value).ConfigureAwait(false);

                if (!isKnown)
                    return Constants.UnknownValue;

                if (isSecret)
                {
                    return new Dictionary<string, object?>
                    {
                        { Constants.SpecialSigKey, Constants.SpecialSecretSig },
                        { "value", value },
                    };
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
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Hit dictionary");
                }

                var result = new Dictionary<string, object>();
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

                return result;
            }

            if (prop is IList list)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Hit list");
                }

                var result = new List<object?>(list.Count);
                for (int i = 0, n = list.Count; i < n; i++)
                {
                    if (_excessiveDebugOutput)
                    {
                        Log.Debug($"Serialize property[{ctx}]: array[{i}] element");
                    }

                    result[i] = await SerializeAsync($"{ctx}[{i}]", list[i]).ConfigureAwait(false);
                }
            }

            throw new InvalidOperationException($"{prop.GetType().FullName} is not a supported argument type.\n\t{ctx}");
        }
    }
}
