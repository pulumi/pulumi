// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumi;
using Pulumi.Aws.S3;

class Program
{
    static Task<int> Main()
        => Deployment.RunAsync(() =>
        {
            var config = new Config("hello-dotnet");
            var name = config.Require("name");

            // Create the bucket, and make it public.
            var bucket = new Bucket(name, new BucketArgs { Acl = "public-read" });

            // Add some content.
            var content = new BucketObject($"{name}-content", new BucketObjectArgs
            {
                Acl = "public-read",
                Bucket = bucket.Id,
                ContentType = "text/plain; charset=utf8",
                Key = "hello.txt",
                Source = new StringAsset("Made with ❤, Pulumi, and .NET"),
            });

            // Return some values that will become the Outputs of the stack.
            return new Dictionary<string, object>
            {
                { "hello", "world" },
                { "bucket-id", bucket.Id },
                { "content-id", content.Id },
                { "object-url", Output.Format($"http://{bucket.BucketDomainName}/{content.Key}") },
            };
        });
}
