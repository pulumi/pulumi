// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi
{
    /// <summary>
    /// An automatically generated logical URN, used to stably identify resources. These are created
    /// automatically by Pulumi to identify resources.  They cannot be manually constructed.
    /// </summary>
    public sealed class Urn : IEquatable<Urn>
    {
        internal readonly string Value;

        internal Urn(string value)
            => Value = value ?? throw new ArgumentNullException(nameof(value));

        public override string ToString()
            => Value;

        public override int GetHashCode()
            => Value.GetHashCode(StringComparison.Ordinal);

        public override bool Equals(object? obj)
            => obj is Urn urn && Equals(urn);

        public bool Equals(Urn urn)
            => Value == urn?.Value;

        /// <summary>
        /// Computes a URN from the combination of a resource name, resource type, optional parent,
        /// optional project and optional stack.
        /// </summary>
        /// <returns></returns>
        internal static Output<Urn> Create(
            Input<string> name, Input<string> type,
            Resource? parent = null, Input<Urn>? parentUrn = null,
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
                    parentUrnString => parentUrnString.Value.Substring(
                        0, parentUrnString.Value.LastIndexOf("::", StringComparison.Ordinal)) + "$");
            }
            else
            {
                parentPrefix = Output.Create($"urn:pulumi:{stack ?? Deployment.Instance.StackName}::{project ?? Deployment.Instance.ProjectName}::");
            }

            return Output.Format($"{parentPrefix}{type}::{name}").Apply(value => new Urn(value));
        }

        /// <summary>
        /// inheritedChildAlias computes the alias that should be applied to a child based on an
        /// alias applied to it's parent. This may involve changing the name of the resource in
        /// cases where the resource has a named derived from the name of the parent, and the parent
        /// name changed.
        /// </summary>
        internal static Output<UrnOrAlias> InheritedChildAlias(string childName, string parentName, Input<Urn> parentAlias, string childType)
        {
            var urn = InheritedChildAliasWorker(childName, parentName, parentAlias, childType);
            return urn.Apply(u => (UrnOrAlias)u);
        }

        internal static Output<Urn> InheritedChildAliasWorker(string childName, string parentName, Input<Urn> parentAlias, string childType)
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
                aliasName = parentAlias.ToOutput().Apply(parentAliasUrn =>
                {
                    var parentAliasVal = parentAliasUrn.Value;
                    var parentAliasName = parentAliasVal.Substring(parentAliasVal.LastIndexOf("::", StringComparison.Ordinal) + 2);
                    return parentAliasName + childName.Substring(parentName.Length);
                });
            }

            return Create(aliasName, childType, parentUrn: parentAlias);
        }
    }
}
