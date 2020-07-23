// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Diagnostics.CodeAnalysis;
using Google.Protobuf.Collections;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi
{
    public interface IResourcePackage
    {
    	Resource Construct(string name, string type, IDictionary<string, object?>? args, string urn);
    }

    internal static class ResourcePackages
    {
    	internal static ConcurrentDictionary<string, IResourcePackage> _resourcePackages = new ConcurrentDictionary<string, IResourcePackage>();

    	private static string PackageKey(string name, string version)
    	{
    		return $"{name}@{version}";
    	}

    	internal static bool TryGetResourcePackage(string name, string version, [NotNullWhen(true)] out IResourcePackage? package)
    	{
    		return _resourcePackages.TryGetValue(PackageKey(name, version), out package);
    	}

    	public static void RegisterResourcePackage(string name, string version, IResourcePackage package)
    	{
    		var key = PackageKey(name, version);
    		if (!_resourcePackages.TryAdd(key, package))
    		{
    			throw new InvalidOperationException($"Cannot re-register package {key}.");
    		}
    	}
    }
}
