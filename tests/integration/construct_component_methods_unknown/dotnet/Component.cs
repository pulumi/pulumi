// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using Pulumi;

class Component : ComponentResource
{
    public Component(string name, ComponentResourceOptions? opts = null)
        : base("testcomponent:index:Component", name, ResourceArgs.Empty, opts, remote: true)
    {
    }

    public Output<ComponentGetMessageResult> GetMessage(ComponentGetMessageArgs args)
        => Deployment.Instance.Call<ComponentGetMessageResult>("testcomponent:index:Component/getMessage", args, this);
}

public class ComponentGetMessageArgs : CallArgs
{
    [Input("echo")]
    public Input<string> Echo { get; set; } = null!;
}

[OutputType]
public sealed class ComponentGetMessageResult
{
    public readonly string Message;

    [OutputConstructor]
    private ComponentGetMessageResult(string message)
    {
        Message = message;
    }
}
