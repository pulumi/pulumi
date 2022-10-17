using Pulumi;
using Aws = Pulumi.Aws;

class MyStack : Stack
{
    public MyStack()
    {
        var mybucket = new Aws.S3.Bucket("mybucket", new Aws.S3.BucketArgs
        {
            Website = new Aws.S3.Inputs.BucketWebsiteArgs
            {
                IndexDocument = "index.html",
            },
        });
        var indexhtml = new Aws.S3.BucketObject("indexhtml", new Aws.S3.BucketObjectArgs
        {
            Bucket = mybucket.Id,
            Source = new FileArchive("<h1>Hello, world!</h1>"),
            Acl = "public-read",
            ContentType = "text/html",
        });
        this.BucketEndpoint = mybucket.WebsiteEndpoint.Apply(websiteEndpoint => $"http://{websiteEndpoint}");
    }

    [Output("bucketEndpoint")]
    public Output<string> BucketEndpoint { get; set; }
}
