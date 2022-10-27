using System.Collections.Generic;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var bucket = new List<Aws.S3.Bucket>();
    for (var rangeIndex = 0; rangeIndex < 10; rangeIndex++)
    {
        var range = new { Value = rangeIndex };
        bucket.Add(new Aws.S3.Bucket($"bucket-{range.Value}", new()
        {
            Website = new Aws.S3.Inputs.BucketWebsiteArgs
            {
                IndexDocument = $"index-{range.Value}.html",
            },
        }));
    }
});

