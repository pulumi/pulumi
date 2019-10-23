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
        public Input<string> Acl = default!;
        public Input<string> Bucket = default!;
        public Input<string> ContentBase64;
        public Input<string> ContentType = default!;
        public Input<string> Key = default!;
        public Input<AssetOrArchive> Source = default!;

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("acl", Acl);
            builder.Add("bucket", Bucket);
            builder.Add("contentBase64", ContentBase64);
            builder.Add("contentType", ContentType);
            builder.Add("key", Key);
            builder.Add("source", Source);
        }
    }

    public class Bucket : CustomResource
    {
        [Property("bucketDomainName")] public Output<string> BucketDomainName { get; private set; }

        public Bucket(string name, BucketArgs args, ResourceOptions options = null)
            : base("aws:s3/bucket:Bucket", name, args, options)
        {
        }
    }

    public class BucketArgs : ResourceArgs
    {
        public Input<string> Acl;

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("acl", Acl);
        }
    }
}
