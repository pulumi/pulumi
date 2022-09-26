package s3point5

import (
	"reflect"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func NewBucketObject(ctx *pulumi.Context, name string, args bucketObjectArgs, opts ...pulumi.ResourceOption) (*s3.BucketObject, error) {
	var resource s3.BucketObject
	err := ctx.RegisterResource("aws:s3/bucketObject:BucketObject", name, args.all, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func NewBucketArgs(bucket pulumi.Promise[string]) bucketObjectArgs {
	return bucketObjectArgs{
		all: allBucketObjectArgs{
			Bucket: bucket,
		},
	}
}

func (args bucketObjectArgs) With(optionalValues OptionalBucketObjectArgs) bucketObjectArgs {
	newArgs := args
	newArgs.all.OptionalBucketObjectArgs = optionalValues
	return newArgs
}

type bucketObjectArgs struct {
	all allBucketObjectArgs
}

type allBucketObjectArgs struct {
	OptionalBucketObjectArgs

	Bucket pulumi.Promise[string] `pulumi:"bucket"`
}

// The set of arguments for constructing a BucketObject resource.
type OptionalBucketObjectArgs struct {
	// [Canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl) to apply. Valid values are `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`, `bucket-owner-read`, and `bucket-owner-full-control`. Defaults to `private`.
	Acl pulumi.Promise[string] `pulumi:"acl"`
	// Whether or not to use [Amazon S3 Bucket Keys](https://docs.aws.amazon.com/AmazonS3/latest/dev/bucket-key.html) for SSE-KMS.
	BucketKeyEnabled pulumi.Promise[bool] `pulumi:"bucketKeyEnabled"`
	// Caching behavior along the request/reply chain Read [w3c cacheControl](http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9) for further details.
	CacheControl pulumi.Promise[string] `pulumi:"cacheControl"`
	// Literal string value to use as the object content, which will be uploaded as UTF-8-encoded text.
	Content pulumi.Promise[string] `pulumi:"content"`
	// Base64-encoded data that will be decoded and uploaded as raw bytes for the object content. This allows safely uploading non-UTF8 binary data, but is recommended only for small content such as the result of the `gzipbase64` function with small text strings. For larger objects, use `source` to stream the content from a disk file.
	ContentBase64 pulumi.Promise[string] `pulumi:"contentBase64"`
	// Presentational information for the object. Read [w3c contentDisposition](http://www.w3.org/Protocols/rfc2616/rfc2616-sec19.html#sec19.5.1) for further information.
	ContentDisposition pulumi.Promise[string] `pulumi:"contentDisposition"`
	// Content encodings that have been applied to the object and thus what decoding mechanisms must be applied to obtain the media-type referenced by the Content-Type header field. Read [w3c content encoding](http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.11) for further information.
	ContentEncoding pulumi.Promise[string] `pulumi:"contentEncoding"`
	// Language the content is in e.g., en-US or en-GB.
	ContentLanguage pulumi.Promise[string] `pulumi:"contentLanguage"`
	// Standard MIME type describing the format of the object data, e.g., application/octet-stream. All Valid MIME Types are valid for this input.
	ContentType pulumi.Promise[string] `pulumi:"contentType"`
	// Triggers updates when the value changes. The only meaningful value is `filemd5("path/to/file")`. This attribute is not compatible with KMS encryption, `kmsKeyId` or `serverSideEncryption = "aws:kms"` (see `sourceHash` instead).
	Etag pulumi.Promise[string] `pulumi:"etag"`
	// Whether to allow the object to be deleted by removing any legal hold on any object version. Default is `false`. This value should be set to `true` only if the bucket has S3 object lock enabled.
	ForceDestroy pulumi.Promise[bool] `pulumi:"forceDestroy"`
	// Name of the object once it is in the bucket.
	Key pulumi.Promise[string] `pulumi:"key"`
	// ARN of the KMS Key to use for object encryption. If the S3 Bucket has server-side encryption enabled, that value will automatically be used. If referencing the `kms.Key` resource, use the `arn` attribute. If referencing the `kms.Alias` data source or resource, use the `targetKeyArn` attribute. This provider will only perform drift detection if a configuration value is provided.
	KmsKeyId pulumi.Promise[string] `pulumi:"kmsKeyId"`
	// Map of keys/values to provision metadata (will be automatically prefixed by `x-amz-meta-`, note that only lowercase label are currently supported by the AWS Go API).
	Metadata pulumi.Promise[map[string]pulumi.Promise[string]] `pulumi:"metadata"`
	// [Legal hold](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-legal-holds) status that you want to apply to the specified object. Valid values are `ON` and `OFF`.
	ObjectLockLegalHoldStatus pulumi.Promise[string] `pulumi:"objectLockLegalHoldStatus"`
	// Object lock [retention mode](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-retention-modes) that you want to apply to this object. Valid values are `GOVERNANCE` and `COMPLIANCE`.
	ObjectLockMode pulumi.Promise[string] `pulumi:"objectLockMode"`
	// Date and time, in [RFC3339 format](https://tools.ietf.org/html/rfc3339#section-5.8), when this object's object lock will [expire](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-retention-periods).
	ObjectLockRetainUntilDate pulumi.Promise[string] `pulumi:"objectLockRetainUntilDate"`
	// Server-side encryption of the object in S3. Valid values are "`AES256`" and "`aws:kms`".
	ServerSideEncryption pulumi.Promise[string] `pulumi:"serverSideEncryption"`
	// Path to a file that will be read and uploaded as raw bytes for the object content.
	Source pulumi.Promise[pulumi.AssetOrArchive] `pulumi:"source"`
	// Triggers updates like `etag` but useful to address `etag` encryption limitations. Set using `filemd5("path/to/source")`. (The value is only stored in state and not saved by AWS.)
	SourceHash pulumi.Promise[string] `pulumi:"sourceHash"`
	// [Storage Class](https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html#AmazonS3-PutObject-request-header-StorageClass) for the object. Defaults to "`STANDARD`".
	StorageClass pulumi.Promise[string] `pulumi:"storageClass"`
	// Map of tags to assign to the object. If configured with a provider `defaultTags` configuration block present, tags with matching keys will overwrite those defined at the provider-level.
	Tags pulumi.Promise[map[string]pulumi.Promise[string]] `pulumi:"tags"`
	// Target URL for [website redirect](http://docs.aws.amazon.com/AmazonS3/latest/dev/how-to-page-redirect.html).
	WebsiteRedirect pulumi.Promise[string] `pulumi:"websiteRedirect"`
}

func (allBucketObjectArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*allBucketObjectArgs)(nil)).Elem()
}

func (OptionalBucketObjectArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*OptionalBucketObjectArgs)(nil)).Elem()
}
