// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Rpc;

namespace Pulumi
{
    public partial class Deployment
    {
        internal static bool _excessiveDebugOutput;

        /// <summary>
        /// serializeResourceProperties walks the props object passed in, awaiting all interior
        /// promises besides those for <see cref="Resource.Urn"/> and <see
        /// cref="CustomResource.Id"/>, creating a reasonable POCO object that can be remoted over
        /// to registerResource.
        /// </summary>
        private static Task<SerializationResult> SerializeResourcePropertiesAsync(
            string label, IDictionary<string, IInput> args)
        {
            return SerializeFilteredPropertiesAsync(
                label, Output.Create(args), key => key != "id" && key != "urn");
        }

        private static async Task<Struct> SerializeAllPropertiesAsync(
            string label, Input<IDictionary<string, IInput>> args)
        {
            var result = await SerializeFilteredPropertiesAsync(label, args, _ => true).ConfigureAwait(false);
            return result.Serialized;
        }

        /// <summary>
        /// serializeFilteredProperties walks the props object passed in, awaiting all interior
        /// promises for properties with keys that match the provided filter, creating a reasonable
        /// POCO object that can be remoted over to registerResource.
        /// </summary>
        private static async Task<SerializationResult> SerializeFilteredPropertiesAsync(
            string label, Input<IDictionary<string, IInput>> args, Predicate<string> acceptKey)
        {
            var props = await args.ToOutput().GetValueAsync().ConfigureAwait(false);

            var propertyToDependentResources = new Dictionary<string, HashSet<Resource>>();
            var result = new Dictionary<string, object>();

            foreach (var (key, input) in props)
            {
                if (acceptKey(key))
                {
                    // We treat properties with null values as if they do not exist.
                    var dependentResources = new HashSet<Resource>();
                    var v = await SerializePropertyAsync($"{label}.{key}", input, dependentResources).ConfigureAwait(false);
                    if (v != null)
                    {
                        result[key] = v;
                        propertyToDependentResources[key] = dependentResources;
                    }
                }
            }

            return new SerializationResult(
                CreateStruct(result.ToImmutableDictionary()),
                propertyToDependentResources.ToImmutableDictionary());
        }

        private static async Task<object?> SerializePropertyAsync(
            string ctx, object? prop, HashSet<Resource> dependentResources)
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

            if (prop is Id id)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: id={id.Value}");
                }

                return id.Value;
            }

            if (prop is Urn urn)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: urn={urn.Value}");
                }

                return urn.Value;
            }

            if (prop is ResourceArgs args)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Recursing into ResourceArgs");
                }

                return await SerializePropertyAsync(ctx, args.ToDictionary(), dependentResources);
            }

            if (prop is AssetOrArchive assetOrArchive)
            {
                var (sig, propName, propValue) = assetOrArchive.GetSerializationData();
                var result = new Dictionary<string, object?>
                {
                    { Constants.SpecialSigKey, sig },
                };

                result[propName] = await SerializePropertyAsync(
                    ctx + "." + propName, propValue, dependentResources).ConfigureAwait(false);
                return result;
            }

            if (prop is Task)
            {
                throw new InvalidOperationException(
$"Tasks are not allowed inside ResourceArgs. Please wrap your Task in an Output:\n\t{ctx}");
            }

            if (prop is IInput input)
            {
                return await SerializePropertyAsync(ctx, input.ToOutput(), dependentResources);
            }

            if (prop is IOutput output)
            {
                if (_excessiveDebugOutput)
                {
                    Log.Debug($"Serialize property[{ctx}]: Recursing into Output");
                }

                dependentResources.AddRange(output.Resources);
                var data = await output.GetDataAsync().ConfigureAwait(false);

                // When serializing an Output, we will either serialize it as its resolved value or the "unknown value"
                // sentinel. We will do the former for all outputs created directly by user code (such outputs always
                // resolve isKnown to true) and for any resource outputs that were resolved with known values.
                var isKnown = data.IsKnown;
                var isSecret = data.IsSecret;
                var value = await SerializePropertyAsync($"{ctx}.id", data.Value, dependentResources);

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

                dependentResources.Add(customResource);
                return await SerializePropertyAsync($"{ctx}.id", customResource.Id, dependentResources);
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

                return await SerializePropertyAsync($"{ctx}.urn", componentResource.Urn, dependentResources);
            }

            if (prop is IDictionary dictionary)
            {
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
                    var v = await SerializePropertyAsync(
                        $"{ctx}.{stringKey}", dictionary[stringKey], dependentResources).ConfigureAwait(false);
                    if (v != null)
                    {
                        result[stringKey] = v;
                    }
                }

                return result;
            }

            if (prop is IList list)
            {
                var result = new List<object?>(list.Count);
                for (int i = 0, n = list.Count; i < n; i++)
                {
                    if (_excessiveDebugOutput)
                    {
                        Log.Debug($"Serialize property[{ctx}]: array[{i}] element");
                    }

                    result[i] = await SerializePropertyAsync($"{ctx}[{i}]", list[i], dependentResources);
                }
            }

            throw new InvalidOperationException($"{prop.GetType().FullName} is not a supported argument type.\n\t{ctx}");
        }

        private struct SerializationResult
        {
            public readonly Struct Serialized;
            public readonly ImmutableDictionary<string, HashSet<Resource>> PropertyToDependentResources;

            public SerializationResult(
                Struct result,
                ImmutableDictionary<string, HashSet<Resource>> propertyToDependentResources)
            {
                Serialized = result;
                PropertyToDependentResources = propertyToDependentResources;
            }

            public void Deconstruct(
                out Struct serialized,
                out ImmutableDictionary<string, HashSet<Resource>> propertyToDependentResources)
            {
                serialized = Serialized;
                propertyToDependentResources = PropertyToDependentResources;
            }
        }
    }
}