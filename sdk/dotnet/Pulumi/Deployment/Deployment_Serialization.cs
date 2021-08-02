// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;

namespace Pulumi
{
    public partial class Deployment
    {
        internal static bool _excessiveDebugOutput = false;

        /// <summary>
        /// <see cref="SerializeResourcePropertiesAsync"/> walks the props object passed in,
        /// awaiting all interior promises besides those for <see cref="Resource.Urn"/> and <see
        /// cref="CustomResource.Id"/>, creating a reasonable POCO object that can be remoted over
        /// to registerResource.
        /// </summary>
        private static Task<SerializationResult> SerializeResourcePropertiesAsync(
            string label, IDictionary<string, object?> args, bool keepResources)
        {
            return SerializeFilteredPropertiesAsync(
                label, args, key => key != Constants.IdPropertyName && key != Constants.UrnPropertyName, keepResources);
        }

        private static async Task<Struct> SerializeAllPropertiesAsync(
            string label, IDictionary<string, object?> args, bool keepResources)
        {
            var result = await SerializeFilteredPropertiesAsync(label, args, _ => true, keepResources).ConfigureAwait(false);
            return result.Serialized;
        }

        /// <summary>
        /// <see cref="SerializeFilteredPropertiesAsync"/> walks the props object passed in,
        /// awaiting all interior promises for properties with keys that match the provided filter,
        /// creating a reasonable POCO object that can be remoted over to registerResource.
        /// </summary>
        private static async Task<SerializationResult> SerializeFilteredPropertiesAsync(
            string label, IDictionary<string, object?> args, Predicate<string> acceptKey, bool keepResources)
        {
            var propertyToDependentResources = ImmutableDictionary.CreateBuilder<string, HashSet<Resource>>();
            var result = ImmutableDictionary.CreateBuilder<string, object>();

            foreach (var (key, val) in args)
            {
                if (acceptKey(key))
                {
                    // We treat properties with null values as if they do not exist.
                    var serializer = new Serializer(_excessiveDebugOutput);
                    var v = await serializer.SerializeAsync($"{label}.{key}", val, keepResources).ConfigureAwait(false);
                    if (v != null)
                    {
                        result[key] = v;
                        propertyToDependentResources[key] = serializer.DependentResources;
                    }
                }
            }

            return new SerializationResult(
                Serializer.CreateStruct(result.ToImmutable()),
                propertyToDependentResources.ToImmutable());
        }

        private readonly struct SerializationResult
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
