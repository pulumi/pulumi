using Pulumi;

class TestResourceArgs : Pulumi.ResourceArgs
{
    [Input("echo")]
    public Input<object>? Echo { get; set; }
}

class TestResource : Pulumi.CustomResource
{
    [Output("echo")]
    public Output<object> Echo { get; private set; } = null!;

    public TestResource(string name, TestResourceArgs args, CustomResourceOptions opts = null)
        : base("testprovider:index:Resource", name, args, opts)
    {
    }
}
