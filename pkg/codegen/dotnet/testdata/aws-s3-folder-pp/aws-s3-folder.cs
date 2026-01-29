using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    // Create a bucket and expose a website index document
    var siteBucket = new Aws.S3.Bucket("siteBucket", new()
    {
        Website = new Aws.S3.Inputs.BucketWebsiteArgs
        {
            IndexDocument = "index.html",
        },
    });

    var siteDir = "www";

    // For each file in the directory, create an S3 object stored in `siteBucket`
    var files = new List<Aws.S3.BucketObject>();
    foreach (var range in Directory.GetFiles(siteDir).Select(Path.GetFileName).Select((v, k) => new { Key = k, Value = v }))
    {
        files.Add(new Aws.S3.BucketObject($"files-{range.Key}", new()
        {
            Bucket = siteBucket.Id,
            Key = range.Value,
            Source = new FileAsset($"{siteDir}/{range.Value}"),
            ContentType = range.Value,
        }, new CustomResourceOptions
        {
            DeletedWith = siteBucket,
        }));
    }
    // set the MIME type of the file
    // Set the access policy for the bucket so all objects are readable
    var bucketPolicy = new Aws.S3.BucketPolicy("bucketPolicy", new()
    {
        Bucket = siteBucket.Id,
        Policy = Output.JsonSerialize(Output.Create(new Dictionary<string, object?>
        {
            ["Version"] = "2012-10-17",
            ["Statement"] = new[]
            {
                new Dictionary<string, object?>
                {
                    ["Effect"] = "Allow",
                    ["Principal"] = "*",
                    ["Action"] = new[]
                    {
                        "s3:GetObject",
                    },
                    ["Resource"] = new[]
                    {
                        siteBucket.Id.Apply(id => $"arn:aws:s3:::{id}/*"),
                    },
                },
            },
        })),
    });

    return new Dictionary<string, object?>
    {
        ["bucketName"] = siteBucket.BucketName,
        ["websiteUrl"] = siteBucket.WebsiteEndpoint,
    };
});

