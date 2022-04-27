// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

using Pulumi;

class MyStack : Stack
{
    public MyStack()
    {
        var customA = new TestResource("a", new TestResourceArgs { Echo = 42 });
    }
}
