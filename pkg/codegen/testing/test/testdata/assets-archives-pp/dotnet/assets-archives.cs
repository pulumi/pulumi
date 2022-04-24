using Pulumi;
using Aws = Pulumi.Aws;

class MyStack : Stack
{
    public MyStack()
    {
        var siteBucket = new Aws.S3.Bucket("siteBucket", new Aws.S3.BucketArgs
        {
        });
        var testFileAsset = new Aws.S3.BucketObject("testFileAsset", new Aws.S3.BucketObjectArgs
        {
            Bucket = siteBucket.Id,
            Source = new FileAsset("file.txt"),
        });
        var testStringAsset = new Aws.S3.BucketObject("testStringAsset", new Aws.S3.BucketObjectArgs
        {
            Bucket = siteBucket.Id,
            Source = new StringAsset("<h1>File contents</h1>"),
        });
        var testRemoteAsset = new Aws.S3.BucketObject("testRemoteAsset", new Aws.S3.BucketObjectArgs
        {
            Bucket = siteBucket.Id,
            Source = new RemoteAsset("https://pulumi.test"),
        });
        var testFileArchive = new Aws.S3.BucketObject("testFileArchive", new Aws.S3.BucketObjectArgs
        {
            Bucket = siteBucket.Id,
            Source = new FileArchive("file.tar.gz"),
        });
        var testRemoteArchive = new Aws.S3.BucketObject("testRemoteArchive", new Aws.S3.BucketObjectArgs
        {
            Bucket = siteBucket.Id,
            Source = new RemoteArchive("https://pulumi.test/foo.tar.gz"),
        });
        var testAssetArchive = new Aws.S3.BucketObject("testAssetArchive", new Aws.S3.BucketObjectArgs
        {
            Bucket = siteBucket.Id,
            Source = new AssetArchive(new Dictionary<string, object?>
            {
                { "file.txt", new FileAsset("file.txt") },
                { "string.txt", new StringAsset("<h1>File contents</h1>") },
                { "remote.txt", new RemoteAsset("https://pulumi.test") },
                { "file.tar", new FileArchive("file.tar.gz") },
                { "remote.tar", new RemoteArchive("https://pulumi.test/foo.tar.gz") },
                { ".nestedDir", new AssetArchive(new Dictionary<string, object?>
                {
                    { "file.txt", new FileAsset("file.txt") },
                    { "string.txt", new StringAsset("<h1>File contents</h1>") },
                    { "remote.txt", new RemoteAsset("https://pulumi.test") },
                    { "file.tar", new FileArchive("file.tar.gz") },
                    { "remote.tar", new RemoteArchive("https://pulumi.test/foo.tar.gz") },
                }) },
            }),
        });
    }

}
