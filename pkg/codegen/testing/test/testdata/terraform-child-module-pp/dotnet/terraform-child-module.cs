using Aws_s3_static_website_bucket;
using System.Collections.Generic;
using Pulumi;
using Aws = Pulumi.Aws;

public class Index : ComponentResource
{
    public Index(string name, IndexArgs args, ComponentResourceOptions? options = null)
        : base("pkg:index:component", name, options)
    {
        // Local Terraform Child Module
        var s3Bucket = new Aws.S3.Bucket("s3Bucket", new()
        {
            BucketName = "s3BucketId",
        });

        var childModule = new Aws_s3_static_website_bucket.Index("childModule", new()
        {
            CustomModuleParameter = s3Bucket.Id,
        });

    }
}

public class IndexArgs
{
}
