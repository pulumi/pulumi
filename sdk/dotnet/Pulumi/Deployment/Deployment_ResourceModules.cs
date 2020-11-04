// Copyright 2016-2020, Pulumi Corporation

using System;
using Pulumi.Serialization;

namespace Pulumi
{
    public partial class Deployment
    {
        public static void RegisterResourceModule(string name, string version, IResourceModule module)
        {
            ResourceModules.RegisterResourceModule(name, version, module);
        }
    }
}
