// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using System;
using Pulumi;

class MyStack : Stack
{
    public MyStack()
    {
        var component = new Component("component");
        var result = component.GetMessage(new ComponentGetMessageArgs { Echo = "hello" });
    }
}
