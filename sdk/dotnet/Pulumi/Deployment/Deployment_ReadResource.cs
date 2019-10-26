// Copyright 2016-2018, Pulumi Corporation

using System;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        private async Task<(string urn, string id, Struct data)> ReadResourceAsync(
            Resource resource, string id, ResourceArgs args, ResourceOptions options)
        {
            var name = resource.GetResourceName();
            var type = resource.GetResourceType();
            var label = $"resource:{name}[{type}]#...";
            Log.Debug($"Reading resource: id={id}, t=${type}, name=${name}");

            var prepareResult = await this.PrepareResourceAsync(
                label, resource, custom: true, args, options).ConfigureAwait(false);

            var serializer = new Serializer(_excessiveDebugOutput);
            Log.Debug($"ReadResource RPC prepared: id={id}, t={type}, name={name}" +
                (_excessiveDebugOutput ? $", obj={prepareResult.SerializedProps}" : ""));

            // Create a resource request and do the RPC.
            var request = new ReadResourceRequest
            {
                Type = type,
                Name = name,
                Id = id,
                Parent = prepareResult.ParentUrn,
                Provider = prepareResult.ProviderRef,
                Properties = prepareResult.SerializedProps,
                Version = options?.Version ?? "",
                AcceptSecrets = true,
            };

            request.Dependencies.AddRange(prepareResult.AllDirectDependencyURNs);

            // Now run the operation, serializing the invocation if necessary.
            var response = await this.Monitor.ReadResourceAsync(request);

            return (response.Urn, id, response.Properties);
        }
    }
}
