// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;

namespace Pulumi
{
    public partial class Deployment
    {
        internal static bool _excessiveDebugOutput = true;

        /// <summary>
        /// <see cref="SerializeResourcePropertiesAsync"/> walks the props object passed in,
        /// awaiting all interior promises besides those for <see cref="Resource.Urn"/> and <see
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
        /// <see cref="SerializeFilteredPropertiesAsync"/> walks the props object passed in,
        /// awaiting all interior promises for properties with keys that match the provided filter,
        /// creating a reasonable POCO object that can be remoted over to registerResource.
        /// </summary>
        private static async Task<SerializationResult> SerializeFilteredPropertiesAsync(
            string label, Input<IDictionary<string, IInput>> args, Predicate<string> acceptKey)
        {
            var props = await args.ToOutput().GetValueAsync().ConfigureAwait(false);

            var propertyToDependentResources = ImmutableDictionary.CreateBuilder<string, HashSet<Resource>>();
            var result = ImmutableDictionary.CreateBuilder<string, object>();

            foreach (var (key, input) in props)
            {
                if (acceptKey(key))
                {
                    // We treat properties with null values as if they do not exist.
                    var serializer = new Serializer(_excessiveDebugOutput);
                    var v = await serializer.SerializeAsync($"{label}.{key}", input).ConfigureAwait(false);
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
