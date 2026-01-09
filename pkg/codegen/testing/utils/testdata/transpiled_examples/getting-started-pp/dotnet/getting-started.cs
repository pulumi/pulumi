using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var mybucket = new Aws.S3.Bucket("mybucket", new()
    {
        Website = new Aws.S3.Inputs.BucketWebsiteArgs
        {
            IndexDocument = "index.html",
        },
    });

    var indexhtml = new Aws.S3.BucketObject("indexhtml", new()
    {
        Bucket = mybucket.Id,
        Source = new StringAsset("<h1>Hello, world!</h1>"),
        Acl = "public-read",
        ContentType = "text/html",
    });

    return new Dictionary<string, object?>
    {
        ["bucketEndpoint"] = mybucket.WebsiteEndpoint.Apply(websiteEndpoint => $"http://{websiteEndpoint}"),
    };
});

