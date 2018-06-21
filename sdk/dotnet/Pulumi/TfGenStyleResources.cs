// These are some hand authored resources in the style of what we think we'd generate via `tfgen`.  So we'll get
// the shape right "by hand" and then work on the code-gen to stub everything else out:

using Pulumi;
using Pulumirpc;
using System;
using System.Threading.Tasks;
using System.Collections.Generic;

namespace AWS.S3 {
    public class Bucket : CustomResource {

        // TODO(ellismg): These should be Output<T>.
        public Task<string> BucketDomainName { get; private set; }
        private TaskCompletionSource<string> m_BucketDomainNameCompletionSource;
        public Bucket(string name, BucketArgs args = default(BucketArgs), ResourceOptions opts = default(ResourceOptions))
        {
            m_BucketDomainNameCompletionSource = new TaskCompletionSource<string>();
            BucketDomainName = m_BucketDomainNameCompletionSource.Task;

            RegisterAsync("aws:s3/bucket:Bucket", name, true, new Dictionary<string, object> {
                {"acl", args.Acl},
            }, opts);
        }

        protected override void OnResourceRegistrationCompete(Task<RegisterResourceResponse> resp)
        {
            base.OnResourceRegistrationCompete(resp);

            if (resp.IsCanceled) {
                m_BucketDomainNameCompletionSource.SetCanceled();
            } else if (resp.IsFaulted) {
                m_BucketDomainNameCompletionSource.SetException(resp.Exception);
            }

            var fields = resp.Result.Object.Fields;

            foreach (var kvp in fields) {
                Serilog.Log.Debug("got property {key}", kvp.Key);
            }

            m_BucketDomainNameCompletionSource.SetResult(fields.ContainsKey("bucketDomainName") ? fields["bucketDomainName"].StringValue : null);
        }
    }


    public struct BucketArgs {
        public Input<string> Acl;
    }

    public class BucketObject : CustomResource{
        public BucketObject(string name, BucketObjectArgs args = default(BucketObjectArgs), ResourceOptions opts = default(ResourceOptions)) {
            RegisterAsync("aws:s3/bucketObject:BucketObject", name, true, new Dictionary<string, object> {
                {"acl", args.Acl},
                {"bucket", args.Bucket},
                {"contentBase64", args.ContentBase64},
                {"contentType", args.ContentType},
                {"key", args.Key},
            }, opts);
        }
    }

    public struct BucketObjectArgs {
        public Input<string> Acl;

        // TODO(ellismg): In the typescript projection, we model this as Input<Bucket | string> since we would marshal the CustomResource
        // using just its ID. Not sure how we want to model there here.  For now, just use a Bucket.
        public Input<Bucket> Bucket;
        public Input<string> ContentBase64;
        public Input<string> ContentEncoding;
        public Input<string> ContentType;
        public Input<string> Key;
    }
}