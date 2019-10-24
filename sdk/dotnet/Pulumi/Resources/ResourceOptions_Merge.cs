// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;

namespace Pulumi
{
    public partial class ResourceOptions
    {
        /// <summary>
        /// Takes two ResourceOptions values and produces a new ResourceOptions with the respective
        /// properties of <paramref name="options2"/> merged over the same properties in <paramref
        /// name="options1"/>.  The original options objects will be unchanged.
        /// <para/>
        /// A new instance will always be returned.
        /// <para/>
        /// Conceptually property merging follows these basic rules:
        /// <list type="number">
        /// <item>
        /// If the property is a collection, the final value will be a collection containing the
        /// values from each options object.
        /// </item>
        /// <item>
        /// Simple scaler values from <paramref name="options2"/> (i.e. <see cref="string"/>s,
        /// <see cref="int"/>s, <see cref="bool"/>s) will replace the values of <paramref
        /// name="options1"/>.
        /// </item>
        /// <item>
        /// <see langword="null"/> values in <paramref name="options2"/> will be ignored.
        /// </item>
        /// </list>
        /// </summary>
        public static ResourceOptions Merge(ResourceOptions? options1, ResourceOptions? options2)
        {
            options1 ??= new ResourceOptions();
            options2 ??= new ResourceOptions();

            if ((options1 is CustomResourceOptions && options2 is ComponentResourceOptions) ||
                (options1 is ComponentResourceOptions && options2 is CustomResourceOptions))
            {
                throw new ArgumentException(
                    $"Cannot merge a {nameof(CustomResourceOptions)} and {nameof(ComponentResourceOptions)} together.");
            }

            // make an appropriate copy of both options bag, then the copy of options2 into the copy
            // of options1 and return the copy of options1.
            if (options1 is CustomResourceOptions || options2 is CustomResourceOptions)
            {
                return MergeCustomOptions(
                    CreateCustomResourceOptionsCopy(options1),
                    CreateCustomResourceOptionsCopy(options2));
            }
            else if (options1 is ComponentResourceOptions || options2 is ComponentResourceOptions)
            {
                return MergeComponentOptions(
                    CreateComponentResourceOptionsCopy(options1),
                    CreateComponentResourceOptionsCopy(options2));
            }
            else
            {
                return MergeNormalOptions(
                    CreateResourceOptionsCopy(options1),
                    CreateResourceOptionsCopy(options2));
            }

            static ResourceOptions MergeNormalOptions(ResourceOptions options1, ResourceOptions options2)
            {
                options1.Id = options2.Id ?? options1.Id;
                options1.Parent = options2.Parent ?? options1.Parent;
                options1.Protect = options2.Protect ?? options1.Protect;
                options1.Version = options2.Version ?? options1.Version;
                options1.Provider = options2.Provider ?? options1.Provider;
                options1.CustomTimeouts = options2.CustomTimeouts ?? options1.CustomTimeouts;

                options1.IgnoreChanges.AddRange(options2.IgnoreChanges);
                options1.ResourceTransformations.AddRange(options2.ResourceTransformations);
                options1.Aliases.AddRange(options2.Aliases);
                
                options1.DependsOn = options1.DependsOn.Concat(options2.DependsOn);
                return options1;
            }

            static CustomResourceOptions MergeCustomOptions(CustomResourceOptions options1, CustomResourceOptions options2)
            {
                // first, merge all the normal option values over
                MergeNormalOptions(options1, options2);

                options1.DeleteBeforeReplace = options2.DeleteBeforeReplace ?? options1.DeleteBeforeReplace;
                options1.ImportId = options2.ImportId ?? options1.ImportId;

                options1.AdditionalSecretOutputs.AddRange(options2.AdditionalSecretOutputs);

                return options1;
            }

            static ComponentResourceOptions MergeComponentOptions(ComponentResourceOptions options1, ComponentResourceOptions options2)
            {
                ExpandProviders(options1);
                ExpandProviders(options2);

                // first, merge all the normal option values over
                MergeNormalOptions(options1, options2);

                options1.Providers.AddRange(options2.Providers);

                if (options1.Providers.Count == 1)
                {
                    options1.Provider = options1.Providers[0];
                    options1.Providers.Clear();
                }

                return options1;
            }

            static void ExpandProviders(ComponentResourceOptions options)
            {
                if (options.Provider != null)
                {
                    options.Providers = new List<ProviderResource> { options.Provider };
                    options.Provider = null;
                }
            }
        }
    }
}
