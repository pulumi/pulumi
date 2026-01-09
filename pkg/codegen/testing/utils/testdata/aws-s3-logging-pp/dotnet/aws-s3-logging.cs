using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var logs = new Aws.S3.Bucket("logs");

    var bucket = new Aws.S3.Bucket("bucket", new()
    {
        Loggings = new[]
        {
            new Aws.S3.Inputs.BucketLoggingArgs
            {
                TargetBucket = logs.BucketName,
            },
        },
    });

    return new Dictionary<string, object?>
    {
        ["targetBucket"] = bucket.Loggings.Apply(loggings => loggings[0]?.TargetBucket),
    };
});

