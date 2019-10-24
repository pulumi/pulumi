// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        void IDeploymentInternal.ReadResource(
            Resource resource, ResourceArgs args, ResourceOptions options)
        {
            // ReadResource is called in a fire-and-forget manner.  Make sure we keep track of
            // this task so that the application will not quit until this async work completes.
            //
            // Also, we can only do our work once the constructor for the resource has actually
            // finished.  Otherwise, we might actually read and get the result back *prior* to
            // the object finishing initializing.  Note: this is not a speculative concern. This is
            // something that does happen and has to be accounted for.
            this.RegisterTask(
                $"{nameof(IDeploymentInternal.ReadResource)}: {resource.GetResourceType()}-{resource.GetResourceName()}",
                CompleteResourceAsync(resource, () => ReadResourceAsync(resource, args, options)));
        }

        private async Task<(string urn, string id, Struct data)> ReadResourceAsync(Resource resource, ResourceArgs args, ResourceOptions options)
        {
            var id = options.Id;
            if (options.Id == null)
            {
                throw new InvalidOperationException("Cannot read resource whose options are lacking an ID value");
            }

            var name = resource.GetResourceName();
            var type = resource.GetResourceType();
            var label = $"resource:{name}[{type}]#...";
            Log.Debug($"Reading resource: id={(id is IOutput ? "Output<T>" : id)}, t=${type}, name=${name}");

            var monitor = this.Monitor;
            var prepareResult = await this.PrepareResourceAsync(
                label, resource, custom: true, args, options).ConfigureAwait(false);

            var serializer = new Serializer(_excessiveDebugOutput);
            var resolvedID = (string)(await serializer.SerializeAsync(label, id).ConfigureAwait(false))!;
            Log.Debug($"ReadResource RPC prepared: id={resolvedID}, t={type}, name={name}" +
                (_excessiveDebugOutput ? $", obj={prepareResult.SerializedProps}" : ""));

            // Create a resource request and do the RPC.
            var request = new ReadResourceRequest
            {
                Type = type,
                Name = name,
                Id = resolvedID,
                Parent = prepareResult.ParentUrn,
                Provider = prepareResult.ProviderRef,
                Properties = prepareResult.SerializedProps,
                Version = options?.Version ?? "",
                AcceptSecrets = true,
            };

            foreach (var urn in prepareResult.AllDirectDependencyURNs)
            {
                request.Dependencies.Add(urn);
            }

            // Now run the operation, serializing the invocation if necessary.
            var response = await monitor.ReadResourceAsync(request);

            return (response.Urn, resolvedID, response.Properties);
        }
    }
}
