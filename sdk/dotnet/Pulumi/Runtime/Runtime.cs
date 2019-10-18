// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    internal static class Runtime
    {
        public static async Task RegisterResourceAsync(
            Resource resource, string type, string name, bool custom,
            ImmutableDictionary<string, Input<object>> properties, ResourceOptions opts)
        {
            var rrr = new RegisterResourceRequest();
            rrr.Object
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
//            req.setType(t);
//            req.setName(name);
//            req.setParent(resop.parentURN);
//            req.setCustom(custom);
//            req.setObject(gstruct.Struct.fromJavaScript(resop.serializedProps));
//            req.setProtect(opts.protect);
//            req.setProvider(resop.providerRef);
//            req.setDependenciesList(Array.from(resop.allDirectDependencyURNs));
//            req.setDeletebeforereplace((< any > opts).deleteBeforeReplace || false);
//            req.setDeletebeforereplacedefined((< any > opts).deleteBeforeReplace !== undefined);
//            req.setIgnorechangesList(opts.ignoreChanges || []);
//            req.setVersion(opts.version || "");
//            req.setAcceptsecrets(true);
//            req.setAdditionalsecretoutputsList((< any > opts).additionalSecretOutputs || []);
//            req.setAliasesList(resop.aliases);
//            req.setImportid(resop.import || "");

//            const customTimeouts = new resproto.RegisterResourceRequest.CustomTimeouts();
//            if (opts.customTimeouts != null)
//            {
//                customTimeouts.setCreate(opts.customTimeouts.create);
//                customTimeouts.setUpdate(opts.customTimeouts.update);
//                customTimeouts.setDelete(opts.customTimeouts.delete);
//            }
//            req.setCustomtimeouts(customTimeouts);

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
    }
}
