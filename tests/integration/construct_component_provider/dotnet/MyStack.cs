// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using Pulumi;

class Provider : ProviderResource
{
    [Output("message")]
    public Output<string> Message { get; private set; } = null!;

    public Provider(string name, ProviderArgs args, CustomResourceOptions? options = null)
        : base("testcomponent", name, args, options)
    {
    }
}

public sealed class ProviderArgs : ResourceArgs
{
    [Input("message")]
    public Input<string> Message { get; set; } = null!;
}

class Component : ComponentResource
{
    [Output("message")]
    public Output<string> Message { get; private set; } = null!;

    public Component(string name, ComponentResourceOptions? opts = null)
        : base("testcomponent:index:Component", name, ResourceArgs.Empty, opts, remote: true)
    {
    }
}

class MyStack : Stack
{
    [Output("message")]
    public Output<string> Message { get; private set; }

    public MyStack()
    {
        var component = new Component("mycomponent", new ComponentResourceOptions
        {
            Providers =
            {
                new Provider("myprovider", new ProviderArgs{ Message = "hello world" }),
            }
        });
        Message = component.Message;
    }
}
