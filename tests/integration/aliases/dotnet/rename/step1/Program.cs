// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

using System.Threading.Tasks;
using Pulumi;

class Resource : ComponentResource
{
    public Resource(string name, ComponentResourceOptions options = null)
        : base("my:module:Resource", name, options)
    {
    }
}

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.RunAsync(() => 
        {
            // Scenario #1 - rename a resource
            var res1 = new Resource("res1");
        });
    }
}