// Copyright 2016-2020, Pulumi Corporation

using System;
using Pulumi.Serialization;

namespace Pulumi
{
    public partial class Deployment
    {
		public static void RegisterResourcePackage(string name, string version, IResourcePackage package)
		{
			ResourcePackages.RegisterResourcePackage(name, version, package);
		}
	}
}
