// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using Pulumi;

class Component : Pulumi.ComponentResource
{
    public Component(string name, ComponentArgs? args = null, ComponentResourceOptions? opts = null)
        : base("testcomponent:index:Component", name, args ?? ResourceArgs.Empty, opts, remote: true)
    {
    }
}

public sealed class ComponentArgs : Pulumi.ResourceArgs
{
    [Input("children")]
    public int? Children { get; set; }

    public ComponentArgs()
    {
    }
}
