// Copyright 2016-2020, Pulumi Corporation

using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// <see cref="DependencyResource"/> is a <see cref="Resource"/> that is used to indicate that an
    /// <see cref="Output"/> has a dependency on a particular resource. These resources are only created when dealing
    /// with remote component resources.
    /// </summary>
    internal sealed class DependencyResource : CustomResource
    {
        public DependencyResource(string urn)
            : base(type: "", name: "", args: ResourceArgs.Empty, dependency: true)
        {
            var resources = ImmutableHashSet.Create<Resource>(this);
            var data = OutputData.Create(resources, urn, isKnown: true, isSecret: false);
            this.Urn = new Output<string>(Task.FromResult(data));
        }
    }
}
