// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    /// <summary>
    /// A <see cref="Resource"/> that aggregates one or more other child resources into a higher
    /// level abstraction.The component resource itself is a resource, but does not require custom
    /// CRUD operations for provisioning.
    /// </summary>
    public class ComponentResource : Resource
    {
        /// <summary>
        /// Creates and registers a new component resource.  <paramref name="type"/> is the fully
        /// qualified type token and <paramref name="name"/> is the "name" part to use in creating a
        /// stable and globally unique URN for the object. <c>opts.parent</c> is the optional parent for
        /// this component, and [opts.dependsOn] is an optional list of other resources that this
        /// resource depends on, controlling the order in which we perform resource operations.
        /// </summary>
        public ComponentResource(string type, string name, ResourceOptions? opts = null)
            : base(type, name, custom: false,
                   args: ResourceArgs.Empty,
                   opts ?? new ComponentResourceOptions())
        {
        }

        // registerOutputs registers synthetic outputs that a component has initialized, usually by
        // allocating other child sub-resources and propagating their resulting property values.
        // ComponentResources should always call this at the end of their constructor to indicate that
        // they are done creating child resources.  While not strictly necessary, this helps the
        // experience by ensuring the UI transitions the ComponentResource to the 'complete' state as
        // quickly as possible (instead of waiting until the entire application completes).
        protected void RegisterOutputs(InputMap<string, object>? map = null)
        {
            Deployment.Instance.RegisterResourceOutputs(this, map ?? new InputMap<string, object>());
        }
    }
}
