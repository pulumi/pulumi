// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using System.Threading.Tasks;
using AWS.S3;
using Pulumi;

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.Run(() =>
        {
            var config = new Config("hello-dotnet");

            // Create the bucket, and make it public.
            var bucket = new Bucket(config.Require("name"), new BucketArgs
            {
                Acl = "public-read"
            });

            // Add some content.  We can use contentBase64 for now, but next we'll want to build out the Assets pipeline so we
            // can do a natural thing.
            var content = new BucketObject($"{config.Require("name")}-content", new BucketObjectArgs
            {
                Acl = "public-read",
                Bucket = bucket.Id,
                ContentType = "text/plain; charset=utf8",
                Key = "hello.txt",
                Source = new StringAsset("Made with \u2764, Pulumi, and .NET"),
            });

            //bucket.Id.Apply(id => Console.WriteLine($"Bucket ID id: {id}"));
            //content.Id.Apply(id => Console.WriteLine($"Content ID id: {id}"));
            //bucket.BucketDomainName.Apply(domain => Console.WriteLine($"https://{domain}/hello.txt"));
            return new Dictionary<string, object>
            {
                { "hello", "world" },
                { "bucket-id", bucket.Id },
                { "content-id", content.Id },
                { "object-url", Output.Format($"http://{bucket.BucketDomainName}/hello.txt") },
            };
        });
    }
}