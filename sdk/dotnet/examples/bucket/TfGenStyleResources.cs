// These are some hand authored resources in the style of what we think we'd generate via `tfgen`.  So we'll get
// the shape right "by hand" and then work on the code-gen to stub everything else out:

using Pulumi;
using Pulumi.Rpc;

namespace AWS.S3
{
    public class BucketObject : CustomResource
    {
        public BucketObject(string name, BucketObjectArgs args, ResourceOptions options = null)
            : base("aws:s3/bucketObject:BucketObject", name, args, options)
        {
            OnConstructorCompleted();
        }
    }

    public class BucketObjectArgs : ResourceArgs
    {
        public Input<string> Acl;
        public Input<Id> Bucket;
        public Input<string> ContentBase64;
        public Input<string> ContentType;
        public Input<string> Key;
        public Input<AssetOrArchive> Source;

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
        [ResourceField("bucketDomainName")]
        private readonly StringOutputCompletionSource _bucketDomainName;
        public Output<string> BucketDomainName => _bucketDomainName.Output;

        public Bucket(string name, BucketArgs args, ResourceOptions options = null)
            : base("aws:s3/bucket:Bucket", name, args, options)
        {
            _bucketDomainName = new StringOutputCompletionSource(this);
            this.OnConstructorCompleted();
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
