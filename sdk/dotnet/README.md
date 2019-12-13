# .NET Language Provider (Preview)

A .NET language provider for Pulumi (currently in preview).


## Building and Running

To build, you'll want to install the .NET Core 3.0 SDK or greater, and ensure
`dotnet` is on your path. Once that it does, running `make` in either the root
directory or the `sdk/dotnet` directory will build and install the language
plugin.

Once this is done you can write a Pulumi app written on top of .NET. You can find
many [examples](https://github.com/pulumi/examples) showing how this can be done with C#, F#, or VB.
Your application will need to reference the [Pulumi NuGet package](https://www.nuget.org/packages/Pulumi/)
or the `Pulumi.dll` built above.

Here's a simple example of a Pulumi app written in C# that creates some simple
AWS resources:

```c#
// Copyright 2016-2019, Pulumi Corporation

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
```

Make a Pulumi.yaml file:

```
$ cat Pulumi.yaml

name: hello-dotnet
runtime: dotnet
```

Then, configure it:

```
$ pulumi stack init hello-dotnet
$ pulumi config set name hello-dotnet
$ pulumi config set aws:region us-west-2
```

And finally, preview and update as you would any other Pulumi project.
