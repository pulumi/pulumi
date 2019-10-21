// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi.Rpc
{
    [AttributeUsage(AttributeTargets.Field)]
    public sealed class ResourceFieldAttribute : Attribute 
    {
        public readonly string Name;

        public ResourceFieldAttribute(string name)
        {
            Name = name;
        }
    }
}
