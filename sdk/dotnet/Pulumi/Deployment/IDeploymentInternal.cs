// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi
{
    internal interface IDeploymentInternal : IDeployment
    {
        Options Options { get; }
        string? GetConfig(string fullKey);

        new Stack Stack { get; set; }

        Task DebugAsync(string message, Resource? resource, int? streamId, bool? ephemeral);
        Task InfoAsync(string message, Resource? resource, int? streamId, bool? ephemeral);
        Task WarnAsync(string message, Resource? resource, int? streamId, bool? ephemeral);
        Task ErrorAsync(string message, Resource? resource, int? streamId, bool? ephemeral);

        Task SetRootResourceAsync(Stack stack);

        void ReadResource(Resource resource, ResourceArgs args, ResourceOptions opts);
        void RegisterResource(Resource resource, bool custom, ResourceArgs args, ResourceOptions opts);
        void RegisterResourceOutputs(Resource resource, Output<IDictionary<string, object>> outputs);
    }
}
