// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using System;
using Pulumi;

class MyStack : Stack
{
    [Output("result")]
    public Output<string> Result { get; set; }

    public MyStack()
    {
        var r = new Random("resource", new RandomArgs { Length = 10 });
        var component = new Component("component");

        Result = component.GetMessage(new ComponentGetMessageArgs { Echo = r.Id }).Apply(v =>
        {
            Console.WriteLine("should not run (result)");
            Environment.Exit(1);
            return v.Message;
        });
    }
}
