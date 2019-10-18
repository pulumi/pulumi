// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    internal static class Runtime
    {
        public static async Task RegisterResourceAsync(
            Resource resource, string type, string name, bool custom,
            ImmutableDictionary<string, Input<object>> properties, ResourceOptions opts)
        {
            throw new NotImplementedException();
        }
    }
}
