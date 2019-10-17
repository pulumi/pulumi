//// These are some hand authored resources in the style of what we think we'd generate via `tfgen`.  So we'll get
//// the shape right "by hand" and then work on the code-gen to stub everything else out:

//using Pulumi;
//using Pulumirpc;
//using System;
//using System.Threading.Tasks;
//using System.Collections.Generic;

//namespace AWS.S3 {
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