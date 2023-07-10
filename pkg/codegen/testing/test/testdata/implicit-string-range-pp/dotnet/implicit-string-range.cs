using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var config = new Config();
    // Number of AZs to cover in a given region
    var azCount = config.Get("azCount") ?? "10";
    var bucketsPerAvailabilityZone = new List<Aws.S3.Bucket>();
    for (var rangeIndex = 0; rangeIndex < int.Parse(azCount); rangeIndex++)
    {
        var range = new { Value = rangeIndex };
        bucketsPerAvailabilityZone.Add(new Aws.S3.Bucket($"bucketsPerAvailabilityZone-{range.Value}", new()
        {
            Website = new Aws.S3.Inputs.BucketWebsiteArgs
            {
                IndexDocument = "index.html",
            },
        }));
    }
});

