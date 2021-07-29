// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;

namespace Pulumi
{
    /// <summary>
    /// ResourceOptions is a bag of optional settings that control a resource's behavior.
    /// </summary>
    public abstract partial class ResourceOptions
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
            get => _dependsOn ??= new InputList<Resource>();
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
            get => _ignoreChanges ??= new List<string>();
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
            get => _resourceTransformations ??= new List<ResourceTransformation>();
            set => _resourceTransformations = value;
        }

        /// <summary>
        /// An optional list of aliases to treat this resource as matching.
        /// </summary>
        public List<Input<Alias>> Aliases { get; set; } = new List<Input<Alias>>();

        /// <summary>
        /// The URN of a previously-registered resource of this type to read from the engine.
        /// </summary>
        public string? Urn { get; set; }

        private List<string>? _replaceOnChanges;

        /// <summary>
        /// Changes to any of these property paths will force a replacement.  If this list
        /// includes `"*"`, changes to any properties will force a replacement.  Initialization
        /// errors from previous deployments will require replacement instead of update only if
        /// `"*"` is passed.
        /// </summary>
        public List<string> ReplaceOnChanges
        {
            get => _replaceOnChanges ??= new List<string>();
            set => _replaceOnChanges = value;
        }

        internal abstract ResourceOptions Clone();
    }
}
