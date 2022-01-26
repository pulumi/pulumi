// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;
using System.Linq;

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
            get => _providers ??= new List<ProviderResource>();
            set => _providers = value;
        }

        internal override ResourceOptions Clone()
            => CreateComponentResourceOptionsCopy(this);
        
        /// <summary>
        /// Takes two <see cref="ComponentResourceOptions"/> values and produces a new
        /// <see cref="ComponentResourceOptions"/> with the respective
        /// properties of <paramref name="options2"/> merged over the same properties in <paramref
        /// name="options1"/>. The original options objects will be unchanged.
        /// <para/>
        /// A new instance will always be returned.
        /// <para/>
        /// Conceptually property merging follows these basic rules:
        /// <list type="number">
        /// <item><description>
        /// If the property is a collection, the final value will be a collection containing the
        /// values from each options object.
        /// </description></item>
        /// <item><description>
        /// Simple scalar values from <paramref name="options2"/> (i.e. <see cref="string"/>s,
        /// <see cref="int"/>s, <see cref="bool"/>s) will replace the values of <paramref
        /// name="options1"/>.
        /// </description></item>
        /// <item><description>
        /// <see langword="null"/> values in <paramref name="options2"/> will be ignored.
        /// </description></item>
        /// </list>
        /// </summary>
        public static ComponentResourceOptions Merge(ComponentResourceOptions? options1, ComponentResourceOptions? options2)
        {
            options1 = options1 != null ? CreateComponentResourceOptionsCopy(options1) : new ComponentResourceOptions();
            options2 = options2 != null ? CreateComponentResourceOptionsCopy(options2) : new ComponentResourceOptions();

            // first, merge all the normal option values over
            MergeNormalOptions(options1, options2);

            options1.Providers = MergeProviders(options1.Providers, options2.Providers);

            if (options1.Providers.Count == 1)
            {
                options1.Provider = options1.Providers[0];
                options1.Providers.Clear();
            }

            return options1;

            static List<ProviderResource> MergeProviders(List<ProviderResource> prov1, List<ProviderResource> prov2 )
            {
                var dict = prov1.ToDictionary(p => p.Package, p => p);
                foreach(var p in prov2)
                {
                    dict[p.Package] = p;
                }
                return dict.Values.ToList();
            }
        }
    }
}
