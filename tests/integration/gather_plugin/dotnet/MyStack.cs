using Pulumi;

class MyStack : Stack
{
    public MyStack()
    {
        // Create an AWS resource (S3 Bucket)
        var r = new Random(
            "default", 10, new ComponentResourceOptions{
                PluginDownloadURL = "get.com",
            });
        var provider = new Provider("explicit", new CustomResourceOptions{
                PluginDownloadURL = "get.pulumi/test/providers",
            });
        var e = new Random("explicit", 8, new ComponentResourceOptions{
                Provider = provider,
            });
    }
}

class TestProviderResourceTypeAttribute : Pulumi.ResourceTypeAttribute
{
    public TestProviderResourceTypeAttribute(string type) : base(type, "1.2.3")
    {
    }
}

[TestProviderResourceTypeAttribute("testprovider:index:Random")]
class Random : ComponentResource
{
    public Random(string name, int length, ComponentResourceOptions? opts = null)
        : base("testprovider:index:Random", name, new RandomResourceArgs {Length = length}, opts, remote: false)
    {
    }

    public sealed class RandomResourceArgs : ResourceArgs
    {
        [Input("length")]
        public Input<int>? Length { get; set; }

        public RandomResourceArgs()
        {
        }
    }

    [Output("result")]
    public Output<string> Result {get; private set;} = null!;
}

class Provider : ProviderResource
{
    public Provider(string name, CustomResourceOptions? opts = null)
        :base("testprovider", name, new ProviderArgs(), MakeResourceOptions(opts, ""))
    {
    }

    private static CustomResourceOptions MakeResourceOptions(CustomResourceOptions? options, Input<string>? id)
    {
        var defaultOptions = new CustomResourceOptions
        {
        };
        var merged = CustomResourceOptions.Merge(defaultOptions, options);
        // Override the ID if one was specified for consistency with other language SDKs.
        merged.Id = id ?? merged.Id;
        return merged;
    }

    public sealed class ProviderArgs : Pulumi.ResourceArgs
    {
        public ProviderArgs()
        {
        }
    }
}
