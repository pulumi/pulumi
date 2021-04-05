// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using Pulumi;

class Component : Pulumi.ComponentResource
{
    public Component(string name, ComponentResourceOptions opts = null)
        : base("testcomponent:index:Component", name, ResourceArgs.Empty, opts, remote: true)
    {
    }
}
