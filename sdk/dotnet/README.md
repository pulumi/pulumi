# Experimental .NET Language Provider

An early prototype of a .NET language provider for Pulumi.


## Building and Running

To build, you'll want to install the .NET Core SDK, and ensure `dotnet` is on
your path.  I'm using the 2.1 SDK, but I believe any 2.0+ SDK will work.

`$ dotnet build` should build the relevent libraries.

You'll also need to build the language host, which is written in Golang and
handles launching `pulumi-language-dotnet-exec`.

`$ GOBIN=/opt/pulumi/bin go install ./cmd/pulumi-language-dotnet`

Add the Publish Pulumi.Host and add the folder to you $PATH:

```
$ dotnet publish Pulumi.Host/pulumi-language-dotnet-exec.csproj
$ export PATH=$(go env
GOPATH)/src/github.com/pulumi/pulumi/sdk/dotnet/Pulumi.Host/bin/Debug/netcoreapp2.0/publish:$PATH
```

Write a little sample app as a csharp script.  You have to include the full Path to Pulumi.dll in
your reference.

```
$ cat main.csx 

#r "/home/matell/go/src/github.com/pulumi/pulumi/sdk/dotnet/Pulumi/bin/Debug/netstandard2.0/Pulumi.dll"
using Pulumi;
using System;

Config config = new Config("hello-dotnet");
CustomResource r = new CustomResource("aws:s3/bucket:Bucket", config["name"]);
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
