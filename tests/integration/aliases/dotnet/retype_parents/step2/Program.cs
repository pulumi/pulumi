// Copyright 2016-2022, Pulumi Corporation.  All rights reserved.

using System.Threading.Tasks;
using System.Linq;
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

    private static System.Collections.Generic.List<Pulumi.Input<Pulumi.Alias>> GenerateAliases() {
        return Enumerable.Range(0, 100).Select(i =>
            (Input<Alias>)(new Alias { Type = $"my:module:ComponentSix-v{i}" })
        ).ToList();
    }

    public ComponentSix(string name, ComponentResourceOptions options = null)
        : base("my:module:ComponentSix-v100", name, ComponentResourceOptions.Merge(options, new ComponentResourceOptions
        {
            // Add an alias that references the old type of this resource
            // and then make the base() call with the new type of this resource and the added alias.
            Aliases = GenerateAliases()
        }))
    {
        // The child resource will also pick up an implicit alias due to the new type of the component it is parented to.
        this.resource = new Resource("otherchild", new ComponentResourceOptions { Parent = this });
    }
}

class ComponentSixParent : ComponentResource
{
    private ComponentSix child;

    private static System.Collections.Generic.List<Pulumi.Input<Pulumi.Alias>> GenerateAliases() {
        return Enumerable.Range(0, 10).Select(i =>
            (Input<Alias>)(new Alias { Type = $"my:module:ComponentSixParent-v{i}" })
        ).ToList();
    }

    public ComponentSixParent(string name, ComponentResourceOptions options = null)
        : base("my:module:ComponentSixParent-v10", name, ComponentResourceOptions.Merge(options, new ComponentResourceOptions
        {
            // Add an alias that references the old type of this resource
            // and then make the base() call with the new type of this resource and the added alias.
            Aliases = GenerateAliases()
        }))
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
