// Copyright 2016-2018, Pulumi Corporation

using System;

namespace Pulumi
{
    /// <summary>
    /// An automatically generated logical URN, used to stably identify resources. These are created
    /// automatically by Pulumi to identify resources.  They cannot be manually constructed.
    /// </summary>
    internal static class Urn
    {
        /// <summary>
        /// Computes a URN from the combination of a resource name, resource type, optional parent,
        /// optional project and optional stack.
        /// </summary>
        /// <returns></returns>
        internal static Output<string> Create(
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
                parentPrefix = Output.Create($"urn:pulumi:{stack ?? Deployment.Instance.StackName}::{project ?? Deployment.Instance.ProjectName}::");
            }

            return Output.Format($"{parentPrefix}{type}::{name}");
        }

        /// <summary>
        /// inheritedChildAlias computes the alias that should be applied to a child based on an
        /// alias applied to it's parent. This may involve changing the name of the resource in
        /// cases where the resource has a named derived from the name of the parent, and the parent
        /// name changed.
        /// </summary>
        internal static Output<UrnOrAlias> InheritedChildAlias(string childName, string parentName, Input<string> parentAlias, string childType)
        {
            var urn = InheritedChildAliasWorker(childName, parentName, parentAlias, childType);
            return urn.Apply(u => (UrnOrAlias)u);
        }

        internal static Output<string> InheritedChildAliasWorker(string childName, string parentName, Input<string> parentAlias, string childType)
        {
            // If the child name has the parent name as a prefix, then we make the assumption that
            // it was constructed from the convention of using '{name}-details' as the name of the
            // child resource.  To ensure this is aliased correctly, we must then also replace the
            // parent aliases name in the prefix of the child resource name.
            //
            // For example:
            // * name: "newapp-function"
            // * options.parent.__name: "newapp"
            // * parentAlias: "urn:pulumi:stackname::projectname::awsx:ec2:Vpc::app"
            // * parentAliasName: "app"
            // * aliasName: "app-function"
            // * childAlias: "urn:pulumi:stackname::projectname::aws:s3/bucket:Bucket::app-function"
            var aliasName = Output.Create(childName);
            if (childName!.StartsWith(parentName, StringComparison.Ordinal))
            {
                aliasName = parentAlias.ToOutput().Apply<string>(parentAliasUrn =>
                {
                    return parentAliasUrn.Substring(parentAliasUrn.LastIndexOf("::", StringComparison.Ordinal) + 2)
                    + childName.Substring(parentName.Length);
                });
            }

            return Create(aliasName, childType, parentUrn: parentAlias);
        }
    }
}
