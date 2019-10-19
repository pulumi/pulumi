// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Immutable;
using System.IO.Compression;
using System.Linq;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Rpc;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        internal void RegisterResource(
            Resource resource, string type, string name, bool custom,
            ResourceArgs args, ResourceOptions opts)
        {
            var completionSources = GetOutputCompletionSources(resource);
            var task1 = RegisterResourceAsync(resource, type, name, custom, args, opts, completionSources);
            // RegisterResource is called in a fire-and-forget manner.  Make sure we keep track of
            // this task so that the application will not quit until this async work completes.
            this.RegisterTask(task1);
        }

        private ImmutableDictionary<string, IOutputCompletionSource> GetOutputCompletionSources(
            Resource resource)
        {
            var query = from field in resource.GetType().GetFields(BindingFlags.NonPublic)
                        let attr = field.GetCustomAttribute<ResourceFieldAttribute>()
                        where attr != null
                        select (field, attr);

            var result = ImmutableDictionary.CreateBuilder<string, IOutputCompletionSource>();
            foreach (var (field, attr) in query.ToList())
            {
                var completionSource = (IOutputCompletionSource)field.GetValue(resource);
                result.Add(attr.Name, completionSource);
            }

            result.Add("urn", resource._urn);
            if (resource is CustomResource customResource)
                result.Add("id", customResource._id);

            return result.ToImmutable();
        }

        private async Task RegisterResourceAsync(
            Resource resource, string type, string name, bool custom,
            ResourceArgs args, ResourceOptions opts,
            ImmutableDictionary<string, IOutputCompletionSource> completionSources)
        {
            try
            {
                var response = await RegisterResourceWorkerAsync(
                    resource, type, name, custom, args, opts).ConfigureAwait(false);

                resource._urn.SetResult(response.Urn);
                if (resource is CustomResource customResource)
                    customResource._id.SetResult(response.Id);

                // Go through all our output fields and lookup a corresponding value in the response
                // object.  Allow the output field to deserialize the response.
                foreach (var (fieldName, completionSource) in completionSources)
                {
                    if (completionSource is IProtobufOutputCompletionSource pbCompletionSource &&
                        response.Object.Fields.TryGetValue(fieldName, out var value))
                    {
                        pbCompletionSource.SetResult(value);
                    }
                }
            }
            catch (Exception e)
            {
                // Mark any unresolved output properties with this exception.  That way we don't
                // leave any outstanding tasks sitting around which might cause hangs.
                foreach (var source in completionSources.Values)
                {
                    source.TrySetException(e);
                }
            }
            finally
            {
                // ensure that we've at least resolved all our completion sources.  That way we
                // don't leave any outstanding tasks sitting around which might cause hangs.
                foreach (var source in completionSources.Values)
                {
                    // Didn't get a value for this field.  Resolve it with a default value.
                    // If we're in preview, we'll consider this unknown and in a normal
                    // update we'll consider it known.
                    source.SetDefaultResult(isKnown: !this.Options.DryRun);
                }
            }
        }

        private async Task<RegisterResourceResponse> RegisterResourceWorkerAsync(
            Resource resource, string type, string name, bool custom,
            ResourceArgs args, ResourceOptions opts)
        {
            var label = $"resource:{name}[{type}]";
            Serilog.Log.Debug($"Registering resource: t={type}, name=${name}, custom=${custom}");

            var request = CreateRegisterResourceRequest(type, name, custom, opts);

            var prepareResult = await PrepareResourceAsync(label, resource, type, custom, args, opts).ConfigureAwait(false);
            PopulateRequest(request, prepareResult);

            return await this.Monitor.RegisterResourceAsync(request);
            
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

        private static void PopulateRequest(RegisterResourceRequest request, PrepareResult prepareResult)
        {
            if (prepareResult.ParentUrn != null)
                request.Parent = prepareResult.ParentUrn.Value;

            if (prepareResult.ProviderRef != null)
                request.Provider = prepareResult.ProviderRef;

            foreach (var alias in prepareResult.Aliases)
                request.Aliases.Add(alias.Value);

            foreach (var dep in prepareResult.AllDirectDependencyURNs)
                request.Dependencies.Add(dep.Value);

            foreach (var (key, resourceURNs) in prepareResult.PropertyToDirectDependencyURNs)
            {
                var deps = new RegisterResourceRequest.Types.PropertyDependencies();
                deps.Urns.AddRange(resourceURNs.Select(u => u.Value));
                request.PropertyDependencies.Add(key, deps);
            }

            request.Object = CreateStruct(prepareResult.SerializedProps);
        }

        private static Value CreateValue(object value)
            => value switch
            {
                null => Value.ForNull(),
                int i => Value.ForNumber(i),
                double d => Value.ForNumber(d),
                bool b => Value.ForBool(b),
                string s => Value.ForString(s),
                IList list => Value.ForList(list.OfType<object>().Select(v => CreateValue(v)).ToArray()),
                IDictionary dict => Value.ForStruct(CreateStruct(dict)),
                _ => throw new InvalidOperationException("Unsupported value when converting to protobuf: " + value.GetType().FullName),
            };

        private static Struct CreateStruct(IDictionary dict)
        {
            var result = new Struct();
            foreach (var key in dict.Keys.OfType<string>())
            {
                result.Fields.Add(key, CreateValue(dict[key]));
            }
            return result;
        }

        private static RegisterResourceRequest CreateRegisterResourceRequest(string type, string name, bool custom, ResourceOptions opts)
        {
            var customOpts = opts as CustomResourceOptions;
            var deleteBeforeReplace = customOpts?.DeleteBeforeReplace;
            var importID = customOpts?.Import;

            var request = new RegisterResourceRequest()
            {
                Type = type,
                Name = name,
                Custom = custom,
                Protect = opts.Protect ?? false,
                Version = opts.Version ?? "",
                ImportId = importID?.Value ?? "",
                AcceptSecrets = true,

                CustomTimeouts = new RegisterResourceRequest.Types.CustomTimeouts(),
                DeleteBeforeReplace = deleteBeforeReplace ?? false,
                DeleteBeforeReplaceDefined = deleteBeforeReplace != null,
            };

            if (customOpts != null)
                request.AdditionalSecretOutputs.AddRange(customOpts.AdditionalSecretOutputs);

            request.IgnoreChanges.AddRange(opts.IgnoreChanges);

            if (opts.CustomTimeouts?.Create != null)
                request.CustomTimeouts.Create = opts.CustomTimeouts.Create;

            if (opts.CustomTimeouts?.Delete != null)
                request.CustomTimeouts.Delete = opts.CustomTimeouts.Delete;

            if (opts.CustomTimeouts?.Update != null)
                request.CustomTimeouts.Update = opts.CustomTimeouts.Update;

            return request;
        }

        //private static async Task ResolveOutputsAsync(Resource resource, Task<RegisterResourceResponse> responseTask)
        //{
        //    var query = from f in resource.GetType().GetFields(BindingFlags.NonPublic)
        //                let attr = f.GetCustomAttribute<ResourceFieldAttribute>()
        //                where attr != null
        //                select f;

        //    var fields = query.ToList();

        //    try
        //    {
        //        var response = 
        //    }
        //    catch (Exception e)
        //    {
        //        resource._urn.SetException(e);
        //        if (resource is CustomResource customResource)
        //            customResource._id.SetException(e);

        //        foreach (var field in fields)
        //        {
        //            var completionSource = (IOutputCompletionSource)field.GetValue(resource);
        //            completionSource.SetException(e);
        //        }

        //        throw;
        //    }
        //}
    }
}
