#r "/home/matell/go/src/github.com/pulumi/pulumi/sdk/dotnet/Pulumi/bin/Debug/netstandard2.0/Pulumi.dll"
using Pulumi;
using System;
using System.Text;
using System.Collections.Generic;

Config config = new Config("hello-dotnet");

// Create the bucket, and make it readable.
var bucket = new CustomResource("aws:s3/bucket:Bucket", config["name"],
    new Dictionary<string, object> {
        { "acl", "public-read" }
    }
);


// Add some content.  We can use contentBase64 for now, but next we'll want to build out the Assets pipeline so we
// can do a natural thing.
var content = new CustomResource("aws:s3/bucketObject:BucketObject", $"{config["name"]}-content",
    new Dictionary<string, object> {
        {"acl", "public-read"},
        {"bucket", bucket.Id},
        {"contentBase64", Convert.ToBase64String(Encoding.UTF8.GetBytes("Made with \u2764 and Pulumi"))},
        {"contentType", "text/plain; charset=utf8"},
        {"key", "hello.txt"},
    }
);

Console.WriteLine($"Bucket ID id  :  {bucket.Id.Result}");
Console.WriteLine($"Content ID id : {content.Id.Result}");
