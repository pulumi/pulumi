// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using Pulumi;

class ComponentArgs : ResourceArgs
{
    [Input("first")]
    public Input<string> First { get; set; } = null!;

    [Input("second")]
    public Input<string> Second { get; set; } = null!;
}

class Component : ComponentResource
{
    public Component(string name, ComponentArgs args, ComponentResourceOptions? opts = null)
        : base("testcomponent:index:Component", name, args, opts, remote: true)
    {
    }

    public Output<GetMessageResult> GetMessage(GetMessageArgs args)
        => Deployment.Instance.Call<GetMessageResult>("testcomponent:index:Component/getMessage", args, this);
}

public class GetMessageArgs : CallArgs
{
    [Input("name")]
    public Input<string> Name { get; set; } = null!;
}

[OutputType]
public sealed class GetMessageResult
{
    public readonly string Message;

    [OutputConstructor]
    private GetMessageResult(string message)
    {
        Message = message;
    }
}
