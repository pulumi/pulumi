using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var siteBucket = new Aws.S3.Bucket("siteBucket");

    var testFileAsset = new Aws.S3.BucketObject("testFileAsset", new()
    {
        Bucket = siteBucket.Id,
        Source = new FileAsset("file.txt"),
    });

    var testStringAsset = new Aws.S3.BucketObject("testStringAsset", new()
    {
        Bucket = siteBucket.Id,
        Source = new StringAsset("<h1>File contents</h1>"),
    });

    var testRemoteAsset = new Aws.S3.BucketObject("testRemoteAsset", new()
    {
        Bucket = siteBucket.Id,
        Source = new RemoteAsset("https://pulumi.test"),
    });

    var testFileArchive = new Aws.Lambda.Function("testFileArchive", new()
    {
        Role = siteBucket.Arn,
        Code = new FileArchive("file.tar.gz"),
    });

    var testRemoteArchive = new Aws.Lambda.Function("testRemoteArchive", new()
    {
        Role = siteBucket.Arn,
        Code = new RemoteArchive("https://pulumi.test/foo.tar.gz"),
    });

    var testAssetArchive = new Aws.Lambda.Function("testAssetArchive", new()
    {
        Role = siteBucket.Arn,
        Code = new AssetArchive(new Dictionary<string, AssetOrArchive>
        {
            ["file.txt"] = new FileAsset("file.txt"),
            ["string.txt"] = new StringAsset("<h1>File contents</h1>"),
            ["remote.txt"] = new RemoteAsset("https://pulumi.test"),
            ["file.tar"] = new FileArchive("file.tar.gz"),
            ["remote.tar"] = new RemoteArchive("https://pulumi.test/foo.tar.gz"),
            [".nestedDir"] = new AssetArchive(new Dictionary<string, AssetOrArchive>
            {
                ["file.txt"] = new FileAsset("file.txt"),
                ["string.txt"] = new StringAsset("<h1>File contents</h1>"),
                ["remote.txt"] = new RemoteAsset("https://pulumi.test"),
                ["file.tar"] = new FileArchive("file.tar.gz"),
                ["remote.tar"] = new RemoteArchive("https://pulumi.test/foo.tar.gz"),
            }),
        }),
    });

});

