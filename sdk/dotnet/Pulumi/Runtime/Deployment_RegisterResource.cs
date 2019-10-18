// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Reflection.Emit;
using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        internal void RegisterResource(
            Resource resource, string type, string name, bool custom,
            ResourceArgs args, ResourceOptions opts)
        {
            this.RegisterTask(RegisterResourceAsync(resource, type, name, custom, args, opts));
        }

        private async Task RegisterResourceAsync(
            Resource resource, string type, string name, bool custom,
            ResourceArgs args, ResourceOptions opts)
        {
            var label = $"resource:{name}[{type}]";
            Serilog.Log.Debug($"Registering resource: t={type}, name=${name}, custom=${custom}");

            var prepareResult = await PrepareResourceAsync(label, resource, type, custom, args, opts).ConfigureAwait(false);

            var customOpts = opts as CustomResourceOptions;
            var deleteBeforeReplace = customOpts?.DeleteBeforeReplace;

            var map = args.ToDictionary();
            var rrr = new RegisterResourceRequest()
            {
                Type = type,
                Name = name,
                Parent = ,
                Custom = custom,
                Protect = opts.Protect ?? false,
                Provider = ,
                Version = opts.Version ?? "",
                CustomTimeouts = new RegisterResourceRequest.Types.CustomTimeouts(),
                DeleteBeforeReplace = deleteBeforeReplace ?? false,
                DeleteBeforeReplaceDefined = deleteBeforeReplace != null,
                AcceptSecrets = true,
                Aliases = ,
                Object = new Google.Protobuf.WellKnownTypes.Struct()
            }

            rrr.IgnoreChanges.AddRange(opts.IgnoreChanges);

            if (customOpts != null) {
                rrr.AdditionalSecretOutputs.AddRange(customOpts.AdditionalSecretOutputs);
            }


            if (opts.CustomTimeouts?.Create != null)
                rrr.CustomTimeouts.Create = opts.CustomTimeouts.Create;

            if (opts.CustomTimeouts?.Delete != null)
                rrr.CustomTimeouts.Delete = opts.CustomTimeouts.Delete;

            if (opts.CustomTimeouts?.Update != null)
                rrr.CustomTimeouts.Update = opts.CustomTimeouts.Update;


            rrr.Object.Fields;
            //            const monitor = getMonitor();
            //            const resopAsync = prepareResource(label, res, custom, props, opts);

            //            // In order to present a useful stack trace if an error does occur, we preallocate potential
            //            // errors here. V8 captures a stack trace at the moment an Error is created and this stack
            //            // trace will lead directly to user code. Throwing in `runAsyncResourceOp` results in an Error
            //            // with a non-useful stack trace.
            //            const preallocError = new Error();
            //            debuggablePromise(resopAsync.then(async (resop) => {
            //            log.debug(`RegisterResource RPC prepared: t =${ t}, name =${ name}` +
            //(excessiveDebugOutput ? `, obj =${ JSON.stringify(resop.serializedProps)}` : ``));

            //            const req = new resproto.RegisterResourceRequest();
            //            req.setParent(resop.parentURN);
            //            req.setObject(gstruct.Struct.fromJavaScript(resop.serializedProps));
            //            req.setProvider(resop.providerRef);
            //            req.setDependenciesList(Array.from(resop.allDirectDependencyURNs));
            //            req.setImportid(resop.import || "");


            //            const propertyDependencies = req.getPropertydependenciesMap();
            //            for (const [key, resourceURNs] of resop.propertyToDirectDependencyURNs) {
            //                const deps = new resproto.RegisterResourceRequest.PropertyDependencies();
            //                deps.setUrnsList(Array.from(resourceURNs));
            //                propertyDependencies.set(key, deps);
            //            }

            //            // Now run the operation, serializing the invocation if necessary.
            //            const opLabel = `monitor.registerResource(${ label})`;
            //            runAsyncResourceOp(opLabel, async () => {
            //            let resp: any;
            //            if (monitor)
            //            {
            //                // If we're running with an attachment to the engine, perform the operation.
            //                resp = await debuggablePromise(new Promise((resolve, reject) =>
            //                    (monitor as any).registerResource(req, (err: grpc.ServiceError, innerResponse: any) => {
            //                        log.debug(`RegisterResource RPC finished: ${label}; err: ${ err}, resp: ${ innerResponse}`);
            //            if (err)
            //            {
            //                // If the monitor is unavailable, it is in the process of shutting down or has already
            //                // shut down. Don't emit an error and don't do any more RPCs, just exit.
            //                if (err.code === grpc.status.UNAVAILABLE)
            //                {
            //                    log.debug("Resource monitor is terminating");
            //                    process.exit(0);
            //                }

            //                // Node lets us hack the message as long as we do it before accessing the `stack` property.
            //                preallocError.message = `failed to register new resource ${ name} [${t}]: ${err.message
            //}`;
            //                            reject(preallocError);
            //                        }
            //                        else {
            //                            resolve(innerResponse);
            //                        }
            //                    })), opLabel);
            //            } else {
            //                // If we aren't attached to the engine, in test mode, mock up a fake response for testing purposes.
            //                const mockurn = await createUrn(req.getName(), req.getType(), req.getParent()).promise();
            //resp = {
            //                    getUrn: () => mockurn,
            //                    getId: () => undefined,
            //                    getObject: () => req.getObject(),
            //                };
            //            }

            //            resop.resolveURN(resp.getUrn());

            //            // Note: 'id || undefined' is intentional.  We intentionally collapse falsy values to
            //            // undefined so that later parts of our system don't have to deal with values like 'null'.
            //            if (resop.resolveID) {
            //                const id = resp.getId() || undefined;
            //resop.resolveID(id, id !== undefined);
            //            }

            //            // Now resolve the output properties.
            //            await resolveOutputs(res, t, name, props, resp.getObject(), resop.resolvers);
            //        });
            //    }), label);

        }

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
            if (custom) {
                var customOpts = opts as CustomResourceOptions;
                importID = customOpts?.Import;
                providerRef = await ProviderResource.RegisterAsync(customOpts?.Provider).ConfigureAwait(false);
            }

            // Collect the URNs for explicit/implicit dependencies for the engine so that it can understand
            // the dependency graph and optimize operations accordingly.

            // The list of all dependencies (implicit or explicit).
            var allDirectDependencies = new HashSet<Resource>(explicitDirectDependencies);

            var allDirectDependencyURNs = await GetAllTransitivelyReferencedCustomResourceURNs(explicitDirectDependencies).ConfigureAwait(false);
            var propertyToDirectDependencyURNs = new Dictionary<string, HashSet<Urn>>();

            foreach (var (propertyName, directDependencies) in propertyToDirectDependencies) {
                AddAll(allDirectDependencies, directDependencies);

                var urns = await GetAllTransitivelyReferencedCustomResourceURNs(directDependencies).ConfigureAwait(false);
                AddAll(allDirectDependencyURNs, urns);
                propertyToDirectDependencyURNs[propertyName] = urns;
            }

            // Wait for all aliases. Note that we use `res.__aliases` instead of `opts.aliases` as the former has been processed
            // in the Resource constructor prior to calling `registerResource` - both adding new inherited aliases and
            // simplifying aliases down to URNs.
            var aliases = new List<Urn>();
            var uniqueAliases = new HashSet<Urn>();
            foreach (var alias in res._aliases) {
                var aliasVal = await alias.ToOutput().GetValueAsync();
                if (!uniqueAliases.Add(aliasVal)) {
                    aliases.Add(aliasVal);
                }
            }

            return new PrepareResult(
                serializedProps,
                parentURN?.Value,
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
    }
}
