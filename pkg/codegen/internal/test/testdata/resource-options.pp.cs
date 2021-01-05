using Pulumi;
using Aws = Pulumi.Aws;

class MyStack : Stack
{
    public MyStack()
    {
        var provider = new Aws.Provider("bucket", new Aws.ProviderArgs
        {
            Region = "us-west-2",
        });
        var bucket1 = new Aws.S3.Bucket("bucket", new Aws.S3.BucketArgs
        {
        }, new CustomResourceOptions
        {
            Provider = provider,
            DependsOn = 
            {
                provider,
            },
            Protect = true,
            IgnoreChanges = 
            {
                "bucket",
                "lifecycleRules[0]",
            },
        });
    }

}
