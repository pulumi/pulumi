// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Reflection.Emit;
using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        private async Task<PrepareResult> PrepareResourceAsync(
            string label, Resource res, string type, bool custom,
            ResourceArgs args, ResourceOptions opts)
        {

            //    // Simply initialize the URN property and get prepared to resolve it later on.
            //    // Note: a resource urn will always get a value, and thus the output property
            //    // for it can always run .apply calls.
            //    let resolveURN: (urn: URN) => void;
            //    (res as any).urn = new Output(
            //        res,
            //        debuggablePromise(
            //            new Promise<URN>(resolve => resolveURN = resolve),
            //            `resolveURN(${ label})`),
            //        /*isKnown:*/ Promise.resolve(true),
            //        /*isSecret:*/ Promise.resolve(false));

            //    // If a custom resource, make room for the ID property.
            //    let resolveID: ((v: any, performApply: boolean) => void) | undefined;
            //    if (custom) {
            //        let resolveValue: (v: ID) => void;
            //        let resolveIsKnown: (v: boolean) => void;
            //        (res as any).id = new Output(
            //            res,
            //            debuggablePromise(new Promise<ID>(resolve => resolveValue = resolve), `resolveID(${ label})`),
            //            debuggablePromise(new Promise<boolean>(
            //                resolve => resolveIsKnown = resolve), `resolveIDIsKnown(${ label})`),
            //            Promise.resolve(false));

            //        resolveID = (v, isKnown) => {
            //            resolveValue(v);
            //        resolveIsKnown(isKnown);
            //    };
            //}

            //// Now "transfer" all input properties into unresolved Promises on res.  This way,
            //// this resource will look like it has all its output properties to anyone it is
            //// passed to.  However, those promises won't actually resolve until the registerResource
            //// RPC returns
            //const resolvers = transferProperties(res, label, props);

            /** IMPORTANT!  We should never await prior to this line, otherwise the Resource will be partly uninitialized. */

            // Before we can proceed, all our dependencies must be finished.
            var explicitDirectDependencies = new HashSet<Resource>(
                await GatherExplicitDependenciesAsync(opts.DependsOn).ConfigureAwait(false));

            // Serialize out all our props to their final values.  In doing so, we'll also collect all
            // the Resources pointed to by any Dependency objects we encounter, adding them to 'propertyDependencies'.
            var (serializedProps, propertyToDirectDependencies) = await SerializeResourcePropertiesAsync(label, args);

            // Wait for the parent to complete.
            // If no parent was provided, parent to the root resource.
            var parentURN = opts.Parent != null
                ? await opts.Parent.Urn.GetValueAsync().ConfigureAwait(false)
                : await GetRootResourceAsync(type).ConfigureAwait(false);

            string? providerRef = null;
            Id? importID = null;
            if (custom)
            {
                var customOpts = opts as CustomResourceOptions;
                importID = customOpts?.Import;
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
                AddAll(allDirectDependencies, directDependencies);

                var urns = await GetAllTransitivelyReferencedCustomResourceURNsAsync(directDependencies).ConfigureAwait(false);
                AddAll(allDirectDependencyURNs, urns);
                propertyToDirectDependencyURNs[propertyName] = urns;
            }

            // Wait for all aliases. Note that we use `res.__aliases` instead of `opts.aliases` as the former has been processed
            // in the Resource constructor prior to calling `registerResource` - both adding new inherited aliases and
            // simplifying aliases down to URNs.
            var aliases = new List<Urn>();
            var uniqueAliases = new HashSet<Urn>();
            foreach (var alias in res._aliases)
            {
                var aliasVal = await alias.ToOutput().GetValueAsync();
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
                aliases,
                importID);
            //    //    resolveURN: resolveURN!,
            //    //resolveID: resolveID,
            //    //resolvers: resolvers,
            //    serializedProps: serializedProps,
            //    parentURN: parentURN,
            //    providerRef: providerRef,
            //    allDirectDependencyURNs: allDirectDependencyURNs,
            //    propertyToDirectDependencyURNs: propertyToDirectDependencyURNs,
            //    aliases: aliases,
            //    import: importID,
            //};
        }

        private Task<ImmutableArray<Resource>> GatherExplicitDependenciesAsync(InputList<Resource> resources)
            => resources.Values.GetValueAsync();

        //    dependsOn: Input<Input<Resource>[]> | Input<Resource> | undefined): Promise<Resource[]> {

        //    if (dependsOn) {
        //        if (Array.isArray(dependsOn)) {
        //            const dos: Resource[] = [];
        //            for (const d of dependsOn) {
        //                dos.push(...(await gatherExplicitDependencies(d)));
        //            }
        //            return dos;
        //        } else if (dependsOn instanceof Promise) {
        //            return gatherExplicitDependencies(await dependsOn);
        //        } else if (Output.isInstance(dependsOn)) {
        //            // Recursively gather dependencies, await the promise, and append the output's dependencies.
        //            const dos = (dependsOn as Output<Input<Resource>[] | Input<Resource>>).apply(v => gatherExplicitDependencies(v));
        //const urns = await dos.promise();
        //const implicits = await gatherExplicitDependencies([...dos.resources()]);
        //            return urns.concat(implicits);
        //        } else {
        //            if (!Resource.isInstance(dependsOn)) {
        //                throw new Error("'dependsOn' was passed a value that was not a Resource.");
        //            }

        //            return [dependsOn];
        //        }
        //    }

        //    return [];
        //}
        //    }

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
            // Cust3]. It will *not* include `Cust4`.

            // To do this, first we just get the transitively reachable set of resources (not diving
            // into custom resources).  In the above picture, if we start with 'Comp1', this will be
            // [Comp1, Cust1, Comp2, Cust2, Cust3]
            var transitivelyReachableResources = GetTransitivelyReferencedChildResourcesOfComponentResources(resources);

            var transitivelyReachableCustomResources = transitivelyReachableResources.OfType<CustomResource>();
            var tasks = transitivelyReachableCustomResources.Select(r => r.Urn.GetValueAsync());
            var urns = await Task.WhenAll(tasks);
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
            public readonly object SerializedProps;
            public readonly Urn? ParentUrn;
            public readonly string? ProviderRef;
            public readonly HashSet<Urn> AllDirectDependencyURNs;
            public readonly Dictionary<string, HashSet<Urn>> PropertyToDirectDependencyURNs;
            public readonly List<Urn> Aliases;
            public readonly Id? ImportID;

            public PrepareResult(object serializedProps, Urn? parentUrn, string? providerRef, HashSet<Urn> allDirectDependencyURNs, Dictionary<string, HashSet<Urn>> propertyToDirectDependencyURNs, List<Urn> aliases, Id? importID)
            {
                SerializedProps = serializedProps;
                ParentUrn = parentUrn;
                ProviderRef = providerRef;
                AllDirectDependencyURNs = allDirectDependencyURNs;
                PropertyToDirectDependencyURNs = propertyToDirectDependencyURNs;
                Aliases = aliases;
                ImportID = importID;
            }
        }
    }
}
