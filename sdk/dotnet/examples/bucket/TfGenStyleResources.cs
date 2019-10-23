// These are some hand authored resources in the style of what we think we'd generate via `tfgen`.  So we'll get
// the shape right "by hand" and then work on the code-gen to stub everything else out:

using Pulumi;
using Pulumi.Serialization;

namespace AWS.S3
{
    public class BucketObject : CustomResource
    {
        public BucketObject(string name, BucketObjectArgs args, ResourceOptions options = null)
            : base("aws:s3/bucketObject:BucketObject", name, args, options)
        {
        }
    }

    public class BucketObjectArgs : ResourceArgs
    {
        [Input("acl")]
        public Input<string> Acl = default!;

        [Input("bucket")]
        public Input<string> Bucket = default!;

        [Input("contentBase64")]
        public Input<string> ContentBase64;

        [Input("contentType")]
        public Input<string> ContentType = default!;

        [Input("key")]
        public Input<string> Key = default!;

        [Input("source")]
        public Input<AssetOrArchive> Source = default!;
    }

    public class Bucket : CustomResource
    {
        [Output("bucketDomainName")] public Output<string> BucketDomainName { get; private set; }

        public Bucket(string name, BucketArgs args, ResourceOptions options = null)
            : base("aws:s3/bucket:Bucket", name, args, options)
        {
        }
    }

    public class BucketArgs : ResourceArgs
    {
        [Input("acl")]
        public Input<string> Acl;
    }
}
