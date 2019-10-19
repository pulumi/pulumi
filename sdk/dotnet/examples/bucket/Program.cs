// Copyright 2016-2018, Pulumi Corporation

using System;
using System.Text;
using Pulumi;

class Program
{
    static void Main(string[] args)
    {
        Deployment.Run(() =>
        {
            Config config = new Config("hello-dotnet");

            // Create the bucket, and make it public.
            var bucket = new Bucket(config["name"], new BucketArgs
            {
                Acl = "public-read"
            });

            // Add some content.  We can use contentBase64 for now, but next we'll want to build out the Assets pipeline so we
            // can do a natural thing.
            var content = new BucketObject($"{config["name"]}-content", new BucketObjectArgs
            {
                Acl = "public-read",
                Bucket = bucket,
                ContentBase64 = Convert.ToBase64String(Encoding.UTF8.GetBytes("Made with \u2764, Pulumi, and .NET")),
                ContentType = "text/plain; charset=utf8",
                Key = "hello.txt"
            });

            // In addition to the logging here being nice, it actually forces us to block until the Tasks that represent the RPC
            // calls to create the resources complete.
            //
            // TODO(ellismg): We need to come up with a solution here. We probably want to track all the pending tasks generated
            // by Pulumi during execution and await them to complete in the host itself...
            bucket.Id.Apply(id => Console.WriteLine($"Bucket ID id: {id}"));
            content.Id.Apply(id => Console.WriteLine($"Content ID id: {id}"));
            bucket.BucketDomainName.Apply(domain => Console.WriteLine($"https://{domain}/hello.txt"));
        });
    }
}