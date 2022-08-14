// Copyright 2016-2022, Pulumi Corporation.  All rights reserved.

using System.Threading.Tasks;
using Pulumi;

class Resource : ComponentResource
{
    public Resource(string name, ComponentResourceOptions options = null)
        : base("my:module:Resource", name, options)
    {
    }
}

// Scenario #6 - Nested parents changing types
class ComponentSix : ComponentResource
{
    private Resource resource;

    public ComponentSix(string name, ComponentResourceOptions options = null)
        : base("my:module:ComponentSix-v0", name, options)
    {
        this.resource = new Resource("otherchild", new ComponentResourceOptions { Parent = this });
    }
}

class ComponentSixParent : ComponentResource
{
    private ComponentSix child;

    public ComponentSixParent(string name, ComponentResourceOptions options = null)
        : base("my:module:ComponentSixParent-v0", name, options)
    {
        this.child = new ComponentSix("child", new ComponentResourceOptions { Parent = this });
    }
}

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.RunAsync(() =>
        {
            var comp6 = new ComponentSixParent("comp6");
        });
    }
}
