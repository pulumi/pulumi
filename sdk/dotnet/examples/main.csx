#r "/home/matell/go/src/github.com/pulumi/pulumi/sdk/dotnet/Pulumi/bin/Debug/netstandard2.0/Pulumi.dll"
using Pulumi;
using System;

Config config = new Config("hello-dotnet");
CustomResource r = new CustomResource("aws:s3/bucket:Bucket", config["name"]);
