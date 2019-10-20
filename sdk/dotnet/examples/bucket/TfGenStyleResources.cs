// These are some hand authored resources in the style of what we think we'd generate via `tfgen`.  So we'll get
// the shape right "by hand" and then work on the code-gen to stub everything else out:

using Pulumi;
using System;
using System.Threading.Tasks;
using System.Collections.Generic;
using Pulumi.Rpc;

namespace AWS.S3
{
    public class BucketObject : CustomResource
    {
        public BucketObject(string name, BucketObjectArgs args, ResourceOptions opts = null)
            : base("aws:s3/bucketObject:BucketObject", name, args, opts)
        {
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

        public Bucket(string name, BucketArgs args, ResourceOptions opts = null)
            : base("aws:s3/bucket:Bucket", name, args, opts)
        {
            Console.WriteLine("Making bucket");
            _bucketDomainName = new StringOutputCompletionSource(this);
            Console.WriteLine("Made bucket");
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
//    public class Bucket : CustomResource {

//        public Output<string> BucketDomainName { get; private set; }
//        private TaskCompletionSource<OutputState<string>> m_BucketDomainNameCompletionSource;
//        public Bucket(string name, BucketArgs args = default(BucketArgs), ResourceOptions opts = default(ResourceOptions))
//        {
//            m_BucketDomainNameCompletionSource = new TaskCompletionSource<OutputState<string>>();
//            BucketDomainName = new Output<string>(m_BucketDomainNameCompletionSource.Task);

//            RegisterAsync("aws:s3/bucket:Bucket", name, true, new Dictionary<string, object> {
//                {"acl", args.Acl},
//            }, opts);
//        }

//        protected override void OnResourceRegistrationComplete(Task<RegisterResourceResponse> resp)
//        {
//            base.OnResourceRegistrationComplete(resp);

//            if (resp.IsCanceled) {
//                m_BucketDomainNameCompletionSource.SetCanceled();
//            } else if (resp.IsFaulted) {
//                m_BucketDomainNameCompletionSource.SetException(resp.Exception);
//            }

//            var fields = resp.Result.Object.Fields;

//            bool isKnown = fields.ContainsKey("bucketDomainName");
//            m_BucketDomainNameCompletionSource.SetResult(new OutputState<string>(isKnown ? fields["bucketDomainName"].StringValue : default(string), isKnown, this));
//        }
//    }


//    public struct BucketArgs {
//        public Input<string> Acl;
//    }

//    public class BucketObject : CustomResource{
//        public BucketObject(string name, BucketObjectArgs args = default(BucketObjectArgs), ResourceOptions opts = default(ResourceOptions)) {
//            RegisterAsync("aws:s3/bucketObject:BucketObject", name, true, new Dictionary<string, object> {
//                {"acl", args.Acl},
//                {"bucket", args.Bucket},
//                {"contentBase64", args.ContentBase64},
//                {"contentType", args.ContentType},
//                {"key", args.Key},
//            }, opts);
//        }
//    }

//    public struct BucketObjectArgs {
//        public Input<string> Acl;

//        // TODO(ellismg): In the typescript projection, we model this as Input<Bucket | string> since we would marshal the CustomResource
//        // using just its ID. Not sure how we want to model there here.  For now, just use a Bucket.
//        public Input<Bucket> Bucket;
//        public Input<string> ContentBase64;
//        public Input<string> ContentEncoding;
//        public Input<string> ContentType;
//        public Input<string> Key;
//    }
//}