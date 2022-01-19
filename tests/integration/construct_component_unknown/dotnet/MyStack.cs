// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using Pulumi;

class MyStack : Stack
{
    public MyStack()
    {
        var resource = new Random("resource", new RandomArgs
        {
            Length = 10,
        });

        var component = new Component("component", new ComponentArgs
        {
            Message = resource.Id.Apply(v => $"message {v}"),
            Nested = new ComponentNestedArgs()
            {
                Value = resource.Id.Apply(v => $"nested.value {v}")
            },
        });
    }
}
