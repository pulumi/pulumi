// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi
{
    /// <summary>
    /// An automatically generated logical URN, used to stably identify resources.
    /// </summary>
    public sealed class Urn
    {
        private readonly string _value;

        private Urn(string value)
            => _value = value;

        /// <summary>
        /// Computes a URN from the combination of a resource name, resource type, optional parent,
        /// optional project and optional stack.
        /// </summary>
        /// <returns></returns>
        public static Output<Urn> Create(
            Input<string> name, Input<string> type,
            Resource? parent, Input<Urn>? parentUrn,
            string? project, string? stack)
        {
            if (parent != null && parentUrn != null)
                throw new ArgumentException("Only one of `parent` and `parentUrn` can be non-null.");

            Output<string> parentPrefix;
            if (parent != null || parentUrn != null)
            {
                var parentUrnOutput = parent != null
                    ? parent.Urn
                    : parentUrn!.ToOutput();

                parentPrefix = parentUrnOutput.Apply(
                    parentUrnString => parentUrnString._value.Substring(0, parentUrnString._value.LastIndexOf("::")) + "$");
            }
            else
            {
                parentPrefix = Output.Create($"urn:pulumi:{stack ?? Stack.Current}::${project ?? Project.Current}::");
            }

            return Output.Format($"{parentPrefix}{type}::{name}").Apply(value => new Urn(value));
        }
    }
}
