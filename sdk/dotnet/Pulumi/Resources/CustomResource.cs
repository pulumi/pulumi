// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// CustomResource is a resource whose create, read, update, and delete(CRUD) operations are
    /// managed by performing external operations on some physical entity. The engine understands
    /// how to diff and perform partial updates of them, and these CRUD operations are implemented
    /// in a dynamically loaded plugin for the defining package.
    /// </summary>
    public class CustomResource : Resource
    {
        /// <summary>
        /// Id is the provider-assigned unique ID for this managed resource.  It is set during
        /// deployments and may be missing (unknown) during planning phases.
        /// </summary>
        [Output("id")]
        public Output<string> Id { get; private set; } = null!;

        /// <summary>
        /// Creates and registers a new managed resource.  t is the fully qualified type token and
        /// name is the "name" part to use in creating a stable and globally unique URN for the
        /// object. dependsOn is an optional list of other resources that this resource depends on,
        /// controlling the order in which we perform resource operations.Creating an instance does
        /// not necessarily perform a create on the physical entity which it represents, and
        /// instead, this is dependent upon the diffing of the new goal state compared to the
        /// current known resource state.
        /// </summary>
        public CustomResource(string type, string name, ResourceArgs args, ResourceOptions? options = null)
            : base(type, name, custom: true, args, options ?? new ResourceOptions())
        {
            if (options is ComponentResourceOptions componentOpts && componentOpts.Providers != null)
            {
                throw new ResourceException("Do not supply 'providers' option to a CustomResource. Did you mean 'provider' instead?", this);
            }
        }
    }
}
