// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;

namespace Pulumi
{
    /// <summary>
    /// A bag of optional settings that control a <see cref="ComponentResource"/>'s behavior.
    /// </summary>
    public sealed class ComponentResourceOptions : ResourceOptions
    {
        private List<ProviderResource>? _providers;

        /// <summary>
        /// An optional set of providers to use for child resources.
        ///
        /// Note: do not provide both <see cref="ResourceOptions.Provider"/> and <see
        /// cref="Providers"/>.
        /// </summary>
        public List<ProviderResource> Providers
        {
            get => _providers ?? (_providers = new List<ProviderResource>());
            set => _providers = value;
        }

        internal override ResourceOptions Clone()
            => CreateComponentResourceOptionsCopy(this);

        /// <inheritdoc cref="ResourceOptions.Merge(ResourceOptions, ResourceOptions)"/>
        public static new ComponentResourceOptions Merge(ResourceOptions? options1, ResourceOptions? options2)
        {
            if (options1 is CustomResourceOptions || options2 is CustomResourceOptions)
                throw new ArgumentException($"{nameof(ComponentResourceOptions)}.{nameof(Merge)} cannot be used to merge {nameof(CustomResourceOptions)}");

            return (ComponentResourceOptions)ResourceOptions.Merge(
                CreateComponentResourceOptionsCopy(options1),
                CreateComponentResourceOptionsCopy(options2));
        }
    }
}
