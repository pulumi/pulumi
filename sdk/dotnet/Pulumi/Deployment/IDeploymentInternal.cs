﻿// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    internal interface IDeploymentInternal : IDeployment
    {
        Options Options { get; }
        string? GetConfig(string fullKey);

        Stack Stack { get; set; }

        ILogger Logger { get; }
        IRunner Runner { get; }

        Task SetRootResourceAsync(Stack stack);

        void ReadOrRegisterResource(Resource resource, ResourceArgs args, ResourceOptions opts);
        void RegisterResourceOutputs(Resource resource, Output<IDictionary<string, object?>> outputs);
    }
}
