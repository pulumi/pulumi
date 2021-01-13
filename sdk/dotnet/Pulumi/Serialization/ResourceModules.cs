// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Diagnostics.CodeAnalysis;
using Google.Protobuf.Collections;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi
{
    public interface IResourceModule
    {
        Resource Construct(string name, string type, IDictionary<string, object?>? args, string urn);
    }

    internal static class ResourceModules
    {
        internal static readonly ConcurrentDictionary<string, IResourceModule> _resourceModules = new ConcurrentDictionary<string, IResourceModule>();

        private static string ModuleKey(string name, string version)
        {
            return $"{name}@{version}";
        }

        internal static bool TryGetResourceModule(string name, string version, [NotNullWhen(true)] out IResourceModule? package)
        {
            return _resourceModules.TryGetValue(ModuleKey(name, version), out package);
        }

        public static void RegisterResourceModule(string name, string version, IResourceModule module)
        {
            var key = ModuleKey(name, version);
            if (!_resourceModules.TryAdd(key, module))
            {
                throw new InvalidOperationException($"Cannot re-register module {key}.");
            }
        }
    }
}
