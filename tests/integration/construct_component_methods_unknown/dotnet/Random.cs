// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using Pulumi;

class RandomArgs : ResourceArgs
{
    [Input("length")]
    public Input<int> Length { get; set; } = null!;
}

class Random : CustomResource
{
    public Random(string name, RandomArgs args, CustomResourceOptions? opts = null)
        : base("testprovider:index:Random", name, args, opts)
    {
    }
}
