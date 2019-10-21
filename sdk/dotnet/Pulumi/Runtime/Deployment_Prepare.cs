// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi
{
    public partial class Deployment
    {
        private async Task<PrepareResult> PrepareResourceAsync(
            string label, Resource res, bool custom,
            ResourceArgs args, ResourceOptions options)
        {
            /* IMPORTANT!  We should never await prior to this line, otherwise the Resource will be partly uninitialized. */

            // Before we can proceed, all our dependencies must be finished.
            var type = res.Type;
            var name = res.Name;

            Log.Debug($"Gathering explicit dependencies: t={type}, name={name}, custom={custom}");
            var explicitDirectDependencies = new HashSet<Resource>(
                await GatherExplicitDependenciesAsync(options.DependsOn).ConfigureAwait(false));
            Log.Debug($"Gathered explicit dependencies: t={type}, name={name}, custom={custom}");

            // Serialize out all our props to their final values.  In doing so, we'll also collect all
            // the Resources pointed to by any Dependency objects we encounter, adding them to 'propertyDependencies'.
            Log.Debug($"Serializing properties: t={type}, name={name}, custom={custom}");
            var (serializedProps, propertyToDirectDependencies) =
                await SerializeResourcePropertiesAsync(label, args.ToDictionary()).ConfigureAwait(false);
            Log.Debug($"Serialized properties: t={type}, name={name}, custom={custom}");

            // Wait for the parent to complete.
            // If no parent was provided, parent to the root resource.
            Log.Debug($"Getting parent urn: t={type}, name={name}, custom={custom}");
            var parentURN = options.Parent != null
                ? await options.Parent.Urn.GetValueAsync().ConfigureAwait(false)
                : await GetRootResourceAsync(type).ConfigureAwait(false);
            Log.Debug($"Got parent urn: t={type}, name={name}, custom={custom}");

            string? providerRef = null;
            if (custom)
            {
                var customOpts = options as CustomResourceOptions;
                providerRef = await ProviderResource.RegisterAsync(customOpts?.Provider).ConfigureAwait(false);
            }

            // Collect the URNs for explicit/implicit dependencies for the engine so that it can understand
            // the dependency graph and optimize operations accordingly.

            // The list of all dependencies (implicit or explicit).
            var allDirectDependencies = new HashSet<Resource>(explicitDirectDependencies);

            var allDirectDependencyURNs = await GetAllTransitivelyReferencedCustomResourceURNsAsync(explicitDirectDependencies).ConfigureAwait(false);
            var propertyToDirectDependencyURNs = new Dictionary<string, HashSet<Urn>>();

            foreach (var (propertyName, directDependencies) in propertyToDirectDependencies)
            {
                allDirectDependencies.AddRange(directDependencies);

                var urns = await GetAllTransitivelyReferencedCustomResourceURNsAsync(directDependencies).ConfigureAwait(false);
                allDirectDependencyURNs.AddRange(urns);
                propertyToDirectDependencyURNs[propertyName] = urns;
            }

            // Wait for all aliases. Note that we use 'res._aliases' instead of 'options.aliases' as
            // the former has been processed in the Resource constructor prior to calling
            // 'registerResource' - both adding new inherited aliases and simplifying aliases down
            // to URNs.
            var aliases = new List<Urn>();
            var uniqueAliases = new HashSet<Urn>();
            foreach (var alias in res._aliases)
            {
                var aliasVal = await alias.ToOutput().GetValueAsync().ConfigureAwait(false);
                if (!uniqueAliases.Add(aliasVal))
                {
                    aliases.Add(aliasVal);
                }
            }

            return new PrepareResult(
                serializedProps,
                parentURN,
                providerRef,
                allDirectDependencyURNs,
                propertyToDirectDependencyURNs,
                aliases);
        }

        private static Task<ImmutableArray<Resource>> GatherExplicitDependenciesAsync(InputList<Resource> resources)
            => resources.ToOutput().GetValueAsync();

        private static async Task<HashSet<Urn>> GetAllTransitivelyReferencedCustomResourceURNsAsync(
            HashSet<Resource> resources)
        {
            // Go through 'resources', but transitively walk through **Component** resources,
            // collecting any of their child resources.  This way, a Component acts as an
            // aggregation really of all the reachable custom resources it parents.  This walking
            // will transitively walk through other child ComponentResources, but will stop when it
            // hits custom resources.  in other words, if we had:
            //
            //              Comp1
            //              /   \
            //          Cust1   Comp2
            //                  /   \
            //              Cust2   Cust3
            //              /
            //          Cust4
            //
            // Then the transitively reachable custom resources of Comp1 will be [Cust1, Cust2,
            // Cust3]. It will *not* include 'Cust4'.

            // To do this, first we just get the transitively reachable set of resources (not diving
            // into custom resources).  In the above picture, if we start with 'Comp1', this will be
            // [Comp1, Cust1, Comp2, Cust2, Cust3]
            var transitivelyReachableResources = GetTransitivelyReferencedChildResourcesOfComponentResources(resources);

            var transitivelyReachableCustomResources = transitivelyReachableResources.OfType<CustomResource>();
            var tasks = transitivelyReachableCustomResources.Select(r => r.Urn.GetValueAsync());
            var urns = await Task.WhenAll(tasks).ConfigureAwait(false);
            return new HashSet<Urn>(urns);
        }

        /// <summary>
        /// Recursively walk the resources passed in, returning them and all resources reachable from
        /// <see cref="Resource.ChildResources"/> through any **Component** resources we encounter.
        /// </summary>
        private static HashSet<Resource> GetTransitivelyReferencedChildResourcesOfComponentResources(HashSet<Resource> resources)
        {
            // Recursively walk the dependent resources through their children, adding them to the result set.
            var result = new HashSet<Resource>();
            AddTransitivelyReferencedChildResourcesOfComponentResources(resources, result);
            return result;
        }

        private static void AddTransitivelyReferencedChildResourcesOfComponentResources(HashSet<Resource> resources, HashSet<Resource> result)
        {
            foreach (var resource in resources)
            {
                if (result.Add(resource))
                {
                    if (resource is ComponentResource)
                    {
                        AddTransitivelyReferencedChildResourcesOfComponentResources(resource.ChildResources, result);
                    }
                }
            }
        }

        private struct PrepareResult
        {
            public readonly Struct SerializedProps;
            public readonly Urn? ParentUrn;
            public readonly string? ProviderRef;
            public readonly HashSet<Urn> AllDirectDependencyURNs;
            public readonly Dictionary<string, HashSet<Urn>> PropertyToDirectDependencyURNs;
            public readonly List<Urn> Aliases;

            public PrepareResult(Struct serializedProps, Urn? parentUrn, string? providerRef, HashSet<Urn> allDirectDependencyURNs, Dictionary<string, HashSet<Urn>> propertyToDirectDependencyURNs, List<Urn> aliases)
            {
                SerializedProps = serializedProps;
                ParentUrn = parentUrn;
                ProviderRef = providerRef;
                AllDirectDependencyURNs = allDirectDependencyURNs;
                PropertyToDirectDependencyURNs = propertyToDirectDependencyURNs;
                Aliases = aliases;
            }
        }
    }
}
