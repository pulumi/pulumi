// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;

namespace Pulumi
{
    internal interface IDeploymentInternal : IDeployment
    {
        string? GetConfig(string fullKey);
        bool IsConfigSecret(string fullKey);

        Stack Stack { get; set; }

        IEngineLogger Logger { get; }
        IRunner Runner { get; }

        void ReadOrRegisterResource(
            Resource resource, bool remote, Func<string, Resource> newDependency, ResourceArgs args,
            ResourceOptions opts);
        void RegisterResourceOutputs(Resource resource, Output<IDictionary<string, object?>> outputs);
    }
}
