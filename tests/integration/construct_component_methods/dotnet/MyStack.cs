// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using Pulumi;

class MyStack : Stack
{
    [Output("message")]
    public Output<string> Message { get; set; }

    public MyStack()
    {
        var component = new Component("component", new ComponentArgs { First = "Hello", Second = "World" });
        var result = component.GetMessage(new GetMessageArgs { Name = "Alice" });
        Message = result.Apply(v => v.Message);
    }
}
