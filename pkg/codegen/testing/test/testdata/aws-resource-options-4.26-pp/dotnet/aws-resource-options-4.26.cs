using System.Collections.Generic;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var provider = new Aws.Provider("provider", new()
    {
        Region = "us-west-2",
    });

    var bucket1 = new Aws.S3.Bucket("bucket1", new()
    {
    }, new CustomResourceOptions
    {
        Provider = provider,
        DependsOn = new[]
        {
            provider,
        },
        Protect = true,
        IgnoreChanges =
        {
            "bucket",
            "lifecycleRules[0]",
        }
    });

});

