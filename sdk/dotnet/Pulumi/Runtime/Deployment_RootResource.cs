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
        private Task<Urn>? _rootResource;

        /// <summary>
        /// returns a root resource URN that will automatically become the default parent of all
        /// resources.  This can be used to ensure that all resources without explicit parents are
        /// parented to a common parent resource.
        /// </summary>
        /// <returns></returns>
        internal Task<Urn?> GetRootResourceAsync(string type)
        {
            if (type == Stack._rootPulumiStackTypeName)
            {
                // We're calling this while creating the stack itself.  No way to know its urn at
                // this point.
                return null;
            }

            return _rootResource ?? throw new InvalidOperationException("Calling GetRootResourceAsync before the root resource was registered!");
        }

        //        /**
        //         * getRootResource returns a root resource URN that will automatically become the default parent of all resources.  This
        //         * can be used to ensure that all resources without explicit parents are parented to a common parent resource.
        //         */
        //        export function getRootResource() : Promise<URN | undefined> {
        //    const engineRef: any = getEngine();
        //    if (!engineRef) {
        //        return Promise.resolve(undefined);
        //    }

        //    const req = new engproto.GetRootResourceRequest();
        //    return new Promise<URN | undefined>((resolve, reject) => {
        //        engineRef.getRootResource(req, (err: grpc.ServiceError, resp: any) => {
        //            // Back-compat case - if the engine we're speaking to isn't aware that it can save and load root resources,
        //            // fall back to the old behavior.
        //            if (err && err.code === grpc.status.UNIMPLEMENTED) {
        //                if (rootResource) {
        //                    rootResource.then(resolve);
        //                    return;
        //                }

        //resolve(undefined);
        //            }

        //            if (err) {
        //                return reject(err);
        //            }

        //            const urn = resp.getUrn();
        //            if (urn) {
        //                return resolve(urn);
        //            }

        //            return resolve(undefined);
        //        });
        //    });
        //}

        internal Task SetRootResourceAsync(Stack stack)
        {
            if (_rootResource != null)
            {
                throw new InvalidOperationException("Tried to set the root resource more than once!");
            }

            _rootResource = SetRootResourceWorkerAsync(stack);
            return _rootResource;
        }

        internal async Task<Urn> SetRootResourceWorkerAsync(Stack stack)
        {
            var resUrn = await stack.Urn.GetValueAsync().ConfigureAwait(false);
            await this.Engine.SetRootResourceAsync(new SetRootResourceRequest
            {
                Urn = resUrn.Value,
            });

            var getResponse = await this.Engine.GetRootResourceAsync(new GetRootResourceRequest());
            return new Urn(getResponse.Urn);
        }

        ///**
        // * setRootResource registers a resource that will become the default parent for all resources without explicit parents.
        // */
        //export async function setRootResource(res: ComponentResource): Promise<void> {
        //    const engineRef: any = getEngine();
        //    if (!engineRef) {
        //        return Promise.resolve();
        //    }

        //    const req = new engproto.SetRootResourceRequest();
        //const urn = await res.urn.promise();
        //req.setUrn(urn);
        //    return new Promise<void>((resolve, reject) => {
        //        engineRef.setRootResource(req, (err: grpc.ServiceError, resp: any) => {
        //            // Back-compat case - if the engine we're speaking to isn't aware that it can save and load root resources,
        //            // fall back to the old behavior.
        //            if (err && err.code === grpc.status.UNIMPLEMENTED) {
        //                rootResource = res.urn.promise();
        //                return resolve();
        //            }

        //            if (err) {
        //                return reject(err);
        //            }

        //            return resolve();
        //        });
        //    });
        //}
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
