// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using System.Linq;

namespace Pulumi
{
    /// <summary>
    /// ResourceOptions is a bag of optional settings that control a resource's behavior.
    /// </summary>
    public class ResourceOptions
    {
        /// <summary>
        /// An optional existing ID to load, rather than create.
        /// </summary>
        public Input<string>? Id { get; set; }

        /// <summary>
        /// An optional parent resource to which this resource belongs.
        /// </summary>
        public Resource? Parent { get; set; }

        private InputList<Resource>? _dependsOn;

        /// <summary>
        /// Optional additional explicit dependencies on other resources.
        /// </summary>
        public InputList<Resource> DependsOn
        {
            get => _dependsOn ?? (_dependsOn = new InputList<Resource>());
            set => _dependsOn = value;
        }

        /// <summary>
        /// When set to true, protect ensures this resource cannot be deleted.
        /// </summary>
        public bool? Protect { get; set; }

        private List<string>? _ignoreChanges;

        /// <summary>
        /// Ignore changes to any of the specified properties.
        /// </summary>
        public List<string> IgnoreChanges
        {
            get => _ignoreChanges ?? (_ignoreChanges = new List<string>());
            set => _ignoreChanges = value;
        }

        /// <summary>
        /// An optional version, corresponding to the version of the provider plugin that should be
        /// used when operating on this resource. This version overrides the version information
        /// inferred from the current package and should rarely be used.
        /// </summary>
        public string? Version { get; set; }

        /// <summary>
        /// An optional provider to use for this resource's CRUD operations. If no provider is
        /// supplied, the default provider for the resource's package will be used. The default
        /// provider is pulled from the parent's provider bag (see also
        /// ComponentResourceOptions.providers).
        ///
        /// If this is a <see cref="ComponentResourceOptions"/> do not provide both <see
        /// cref="Provider"/> and <see cref="ComponentResourceOptions.Providers"/>.
        /// </summary>
        public ProviderResource? Provider { get; set; }

        /// <summary>
        ///  An optional CustomTimeouts configuration block.
        /// </summary>
        public CustomTimeouts? CustomTimeouts { get; set; }

        private List<ResourceTransformation>? _resourceTransformations;

        /// <summary>
        /// Optional list of transformations to apply to this resource during construction.The
        /// transformations are applied in order, and are applied prior to transformation applied to
        /// parents walking from the resource up to the stack.
        /// </summary>
        public List<ResourceTransformation> ResourceTransformations
        {
            get => _resourceTransformations ?? (_resourceTransformations = new List<ResourceTransformation>());
            set => _resourceTransformations = value;
        }

        /// <summary>
        /// An optional list of aliases to treat this resource as matching.
        /// </summary>
        public List<Input<UrnOrAlias>> Aliases { get; set; } = new List<Input<UrnOrAlias>>();

        internal virtual ResourceOptions Clone()
            => new ResourceOptions
            {
                Aliases = this.Aliases.ToList(),
                CustomTimeouts = CustomTimeouts.Clone(this.CustomTimeouts),
                DependsOn = this.DependsOn.Clone(),
                Id = this.Id,
                Parent = this.Parent,
                IgnoreChanges = this.IgnoreChanges.ToList(),
                Protect = this.Protect,
                Provider = this.Provider,
                ResourceTransformations = this.ResourceTransformations.ToList(),
                Version = this.Version,
            };
    }
}
