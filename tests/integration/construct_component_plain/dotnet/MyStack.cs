// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using Pulumi;

class MyStack : Stack
{
    public MyStack()
    {
        new Component("component", new ComponentArgs { Children = 5 });
    }
}
