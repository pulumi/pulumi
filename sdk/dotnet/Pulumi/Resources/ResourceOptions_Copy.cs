// Copyright 2016-2019, Pulumi Corporation

using System.Linq;
using System.Collections.Generic;

namespace Pulumi
{
    public partial class ResourceOptions
    {
        internal static ResourceOptions CreateResourceOptionsCopy(ResourceOptions options)
            => new ResourceOptions
            {
                Aliases = options.Aliases.ToList(),
                CustomTimeouts = CustomTimeouts.Clone(options.CustomTimeouts),
                DependsOn = options.DependsOn.Clone(),
                Id = options.Id,
                Parent = options.Parent,
                IgnoreChanges = options.IgnoreChanges.ToList(),
                Protect = options.Protect,
                Provider = options.Provider,
                ResourceTransformations = options.ResourceTransformations.ToList(),
                Version = options.Version,
            };

        internal static CustomResourceOptions CreateCustomResourceOptionsCopy(ResourceOptions options)
        {
            var customOptions = options as CustomResourceOptions;
            var copied = CreateResourceOptionsCopy(options);

            return new CustomResourceOptions
            {
                // Base properties
                Aliases = copied.Aliases,
                CustomTimeouts = copied.CustomTimeouts,
                DependsOn = copied.DependsOn,
                Id = copied.Id,
                Parent = copied.Parent,
                IgnoreChanges = copied.IgnoreChanges,
                Protect = copied.Protect,
                Provider = copied.Provider,
                ResourceTransformations = copied.ResourceTransformations,
                Version = copied.Version,

                // Our properties
                AdditionalSecretOutputs = customOptions?.AdditionalSecretOutputs.ToList() ?? new List<string>(),
                DeleteBeforeReplace = customOptions?.DeleteBeforeReplace,
                ImportId = customOptions?.ImportId,
            };
        }

        internal static ComponentResourceOptions CreateComponentResourceOptionsCopy(ResourceOptions options)
        {
            var componentOptions = options as ComponentResourceOptions;
            var cloned = CreateResourceOptionsCopy(options);

            return new ComponentResourceOptions
            {
                // Base properties
                Aliases = cloned.Aliases,
                CustomTimeouts = cloned.CustomTimeouts,
                DependsOn = cloned.DependsOn,
                Id = cloned.Id,
                Parent = cloned.Parent,
                IgnoreChanges = cloned.IgnoreChanges,
                Protect = cloned.Protect,
                Provider = cloned.Provider,
                ResourceTransformations = cloned.ResourceTransformations,
                Version = cloned.Version,

                // Our properties
                Providers = componentOptions?.Providers.ToList() ?? new List<ProviderResource>(),
            };
        }
    }
}
