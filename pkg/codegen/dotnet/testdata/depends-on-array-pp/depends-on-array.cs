using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var myBucket = new Aws.S3.Bucket("myBucket", new()
    {
        Website = new Aws.S3.Inputs.BucketWebsiteArgs
        {
            IndexDocument = "index.html",
        },
    });

    var ownershipControls = new Aws.S3.BucketOwnershipControls("ownershipControls", new()
    {
        Bucket = myBucket.Id,
        Rule = new Aws.S3.Inputs.BucketOwnershipControlsRuleArgs
        {
            ObjectOwnership = "ObjectWriter",
        },
    });

    var publicAccessBlock = new Aws.S3.BucketPublicAccessBlock("publicAccessBlock", new()
    {
        Bucket = myBucket.Id,
        BlockPublicAcls = false,
    });

    var indexHtml = new Aws.S3.BucketObject("index.html", new()
    {
        Bucket = myBucket.Id,
        Source = new FileAsset("./index.html"),
        ContentType = "text/html",
        Acl = "public-read",
    }, new CustomResourceOptions
    {
        DependsOn =
        {
            publicAccessBlock,
            ownershipControls,
        },
    });

    return new Dictionary<string, object?>
    {
        ["bucketName"] = myBucket.Id,
        ["bucketEndpoint"] = myBucket.WebsiteEndpoint.Apply(websiteEndpoint => $"http://{websiteEndpoint}"),
    };
});

