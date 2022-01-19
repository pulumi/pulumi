// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using Pulumi;

class Component : Pulumi.ComponentResource
{
    public Component(string name, ComponentArgs args, ComponentResourceOptions? opts = null)
        : base("testcomponent:index:Component", name, args, opts, remote: true)
    {
    }
}

class ComponentArgs : Pulumi.ResourceArgs
{
    [Input("message")]
    public Input<string> Message { get; set; } = null!;
    [Input("nested")]
    public Input<ComponentNestedArgs> Nested { get; set; } = null!;
}

class ComponentNestedArgs : Pulumi.ResourceArgs
{
    [Input("value")]
    public Input<string> Value { get; set; } = null!;
}
