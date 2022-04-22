// Copyright 2016-2022, Pulumi Corporation

using System;

namespace Pulumi
{
    /// <summary>
    /// An automatically generated logical URN, used to stably identify resources. These are created
    /// automatically by Pulumi to identify resources.  They cannot be manually constructed.
    /// </summary>
    public static class Urn
    {
        /// <summary>
        /// Computes a URN from the combination of a resource name, resource type, optional parent,
        /// optional project and optional stack.
        /// </summary>
        /// <returns></returns>
        public static Output<string> Create(
            Input<string> name, Input<string> type,
            Resource? parent = null, Input<string>? parentUrn = null,
            Input<string>? project = null, Input<string>? stack = null)
        {
            if (parent != null && parentUrn != null)
                throw new ArgumentException("Only one of 'parent' and 'parentUrn' can be non-null.");

            Output<string> parentPrefix;
            if (parent != null || parentUrn != null)
            {
                var parentUrnOutput = parent != null
                    ? parent.Urn
                    : parentUrn!.ToOutput();

                parentPrefix = parentUrnOutput.Apply(
                    parentUrnString => parentUrnString.Substring(
                        0, parentUrnString.LastIndexOf("::", StringComparison.Ordinal)) + "$");
            }
            else
            {
                parentPrefix = Output.Format($"urn:pulumi:{stack ?? Deployment.Instance.StackName}::{project ?? Deployment.Instance.ProjectName}::");
            }

            return Output.Format($"{parentPrefix}{type}::{name}");
        }
    }
}
