package s3

import (
	"errors"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type BucketObject struct {
	pulumi.CustomResourceState

	// [Canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl) to apply. Valid values are `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`, `bucket-owner-read`, and `bucket-owner-full-control`. Defaults to `private`.
	Acl pulumi.OutputT[string] `pulumi:"acl"`
	// Name of the bucket to put the file in. Alternatively, an [S3 access point](https://docs.aws.amazon.com/AmazonS3/latest/dev/using-access-points.html) ARN can be specified.
	Bucket pulumi.OutputT[string] `pulumi:"bucket"`
	// Whether or not to use [Amazon S3 Bucket Keys](https://docs.aws.amazon.com/AmazonS3/latest/dev/bucket-key.html) for SSE-KMS.
	BucketKeyEnabled pulumi.OutputT[bool] `pulumi:"bucketKeyEnabled"`
	// Caching behavior along the request/reply chain Read [w3c cacheControl](http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9) for further details.
	CacheControl pulumi.OutputT[string] `pulumi:"cacheControl"`
	// Literal string value to use as the object content, which will be uploaded as UTF-8-encoded text.
	Content pulumi.OutputT[string] `pulumi:"content"`
	// Base64-encoded data that will be decoded and uploaded as raw bytes for the object content. This allows safely uploading non-UTF8 binary data, but is recommended only for small content such as the result of the `gzipbase64` function with small text strings. For larger objects, use `source` to stream the content from a disk file.
	ContentBase64 pulumi.OutputT[string] `pulumi:"contentBase64"`
	// Presentational information for the object. Read [w3c contentDisposition](http://www.w3.org/Protocols/rfc2616/rfc2616-sec19.html#sec19.5.1) for further information.
	ContentDisposition pulumi.OutputT[string] `pulumi:"contentDisposition"`
	// Content encodings that have been applied to the object and thus what decoding mechanisms must be applied to obtain the media-type referenced by the Content-Type header field. Read [w3c content encoding](http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.11) for further information.
	ContentEncoding pulumi.OutputT[string] `pulumi:"contentEncoding"`
	// Language the content is in e.g., en-US or en-GB.
	ContentLanguage pulumi.OutputT[string] `pulumi:"contentLanguage"`
	// Standard MIME type describing the format of the object data, e.g., application/octet-stream. All Valid MIME Types are valid for this input.
	ContentType pulumi.OutputT[string] `pulumi:"contentType"`
	// Triggers updates when the value changes. The only meaningful value is `filemd5("path/to/file")`. This attribute is not compatible with KMS encryption, `kmsKeyId` or `serverSideEncryption = "aws:kms"` (see `sourceHash` instead).
	Etag pulumi.OutputT[string] `pulumi:"etag"`
	// Whether to allow the object to be deleted by removing any legal hold on any object version. Default is `false`. This value should be set to `true` only if the bucket has S3 object lock enabled.
	ForceDestroy pulumi.OutputT[bool] `pulumi:"forceDestroy"`
	// Name of the object once it is in the bucket.
	Key pulumi.OutputT[string] `pulumi:"key"`
	// ARN of the KMS Key to use for object encryption. If the S3 Bucket has server-side encryption enabled, that value will automatically be used. If referencing the `kms.Key` resource, use the `arn` attribute. If referencing the `kms.Alias` data source or resource, use the `targetKeyArn` attribute. This provider will only perform drift detection if a configuration value is provided.
	KmsKeyId pulumi.OutputT[string] `pulumi:"kmsKeyId"`
	// Map of keys/values to provision metadata (will be automatically prefixed by `x-amz-meta-`, note that only lowercase label are currently supported by the AWS Go API).
	Metadata pulumi.MapOutputT[string] `pulumi:"metadata"`
	// [Legal hold](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-legal-holds) status that you want to apply to the specified object. Valid values are `ON` and `OFF`.
	ObjectLockLegalHoldStatus pulumi.OutputT[string] `pulumi:"objectLockLegalHoldStatus"`
	// Object lock [retention mode](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-retention-modes) that you want to apply to this object. Valid values are `GOVERNANCE` and `COMPLIANCE`.
	ObjectLockMode pulumi.OutputT[string] `pulumi:"objectLockMode"`
	// Date and time, in [RFC3339 format](https://tools.ietf.org/html/rfc3339#section-5.8), when this object's object lock will [expire](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-retention-periods).
	ObjectLockRetainUntilDate pulumi.OutputT[string] `pulumi:"objectLockRetainUntilDate"`
	// Server-side encryption of the object in S3. Valid values are "`AES256`" and "`aws:kms`".
	ServerSideEncryption pulumi.OutputT[string] `pulumi:"serverSideEncryption"`
	// Path to a file that will be read and uploaded as raw bytes for the object content.
	Source pulumi.OutputT[pulumi.AssetOrArchive] `pulumi:"source"`
	// Triggers updates like `etag` but useful to address `etag` encryption limitations. Set using `filemd5("path/to/source")`. (The value is only stored in state and not saved by AWS.)
	SourceHash pulumi.OutputT[string] `pulumi:"sourceHash"`
	// [Storage Class](https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html#AmazonS3-PutObject-request-header-StorageClass) for the object. Defaults to "`STANDARD`".
	StorageClass pulumi.OutputT[string] `pulumi:"storageClass"`
	// Map of tags to assign to the object. If configured with a provider `defaultTags` configuration block present, tags with matching keys will overwrite those defined at the provider-level.
	Tags pulumi.MapOutputT[string] `pulumi:"tags"`
	// Map of tags assigned to the resource, including those inherited from the provider `defaultTags` configuration block.
	TagsAll pulumi.MapOutputT[string] `pulumi:"tagsAll"`
	// Unique version ID value for the object, if bucket versioning is enabled.
	VersionId pulumi.OutputT[string] `pulumi:"versionId"`
	// Target URL for [website redirect](http://docs.aws.amazon.com/AmazonS3/latest/dev/how-to-page-redirect.html).
	WebsiteRedirect pulumi.OutputT[string] `pulumi:"websiteRedirect"`
}

func NewBucketObject(ctx *pulumi.Context,
	name string, args *BucketObjectArgs, opts ...pulumi.ResourceOption,
) (*BucketObject, error) {
	if args == nil {
		return nil, errors.New("missing one or more required arguments")
	}

	if args.Bucket == nil {
		return nil, errors.New("invalid value for required argument 'Bucket'")
	}
	var resource BucketObject
	err := ctx.RegisterResource("aws:s3/bucketObject:BucketObject", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type bucketObjectArgs struct {
	// [Canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl) to apply. Valid values are `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`, `bucket-owner-read`, and `bucket-owner-full-control`. Defaults to `private`.
	Acl *string `pulumi:"acl"`
	// Name of the bucket to put the file in. Alternatively, an [S3 access point](https://docs.aws.amazon.com/AmazonS3/latest/dev/using-access-points.html) ARN can be specified.
	Bucket interface{} `pulumi:"bucket"`
	// Whether or not to use [Amazon S3 Bucket Keys](https://docs.aws.amazon.com/AmazonS3/latest/dev/bucket-key.html) for SSE-KMS.
	BucketKeyEnabled *bool `pulumi:"bucketKeyEnabled"`
	// Caching behavior along the request/reply chain Read [w3c cacheControl](http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9) for further details.
	CacheControl *string `pulumi:"cacheControl"`
	// Literal string value to use as the object content, which will be uploaded as UTF-8-encoded text.
	Content *string `pulumi:"content"`
	// Base64-encoded data that will be decoded and uploaded as raw bytes for the object content. This allows safely uploading non-UTF8 binary data, but is recommended only for small content such as the result of the `gzipbase64` function with small text strings. For larger objects, use `source` to stream the content from a disk file.
	ContentBase64 *string `pulumi:"contentBase64"`
	// Presentational information for the object. Read [w3c contentDisposition](http://www.w3.org/Protocols/rfc2616/rfc2616-sec19.html#sec19.5.1) for further information.
	ContentDisposition *string `pulumi:"contentDisposition"`
	// Content encodings that have been applied to the object and thus what decoding mechanisms must be applied to obtain the media-type referenced by the Content-Type header field. Read [w3c content encoding](http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.11) for further information.
	ContentEncoding *string `pulumi:"contentEncoding"`
	// Language the content is in e.g., en-US or en-GB.
	ContentLanguage *string `pulumi:"contentLanguage"`
	// Standard MIME type describing the format of the object data, e.g., application/octet-stream. All Valid MIME Types are valid for this input.
	ContentType *string `pulumi:"contentType"`
	// Triggers updates when the value changes. This attribute is not compatible with KMS encryption, `kmsKeyId` or `serverSideEncryption = "aws:kms"` (see `sourceHash` instead).
	Etag *string `pulumi:"etag"`
	// Whether to allow the object to be deleted by removing any legal hold on any object version. Default is `false`. This value should be set to `true` only if the bucket has S3 object lock enabled.
	ForceDestroy *bool `pulumi:"forceDestroy"`
	// Name of the object once it is in the bucket.
	Key *string `pulumi:"key"`
	// ARN of the KMS Key to use for object encryption. If the S3 Bucket has server-side encryption enabled, that value will automatically be used. If referencing the `kms.Key` resource, use the `arn` attribute. If referencing the `kms.Alias` data source or resource, use the `targetKeyArn` attribute. The provider will only perform drift detection if a configuration value is provided.
	KmsKeyId *string `pulumi:"kmsKeyId"`
	// Map of keys/values to provision metadata (will be automatically prefixed by `x-amz-meta-`, note that only lowercase label are currently supported by the AWS Go API).
	Metadata map[string]string `pulumi:"metadata"`
	// [Legal hold](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-legal-holds) status that you want to apply to the specified object. Valid values are `ON` and `OFF`.
	ObjectLockLegalHoldStatus *string `pulumi:"objectLockLegalHoldStatus"`
	// Object lock [retention mode](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-retention-modes) that you want to apply to this object. Valid values are `GOVERNANCE` and `COMPLIANCE`.
	ObjectLockMode *string `pulumi:"objectLockMode"`
	// Date and time, in [RFC3339 format](https://tools.ietf.org/html/rfc3339#section-5.8), when this object's object lock will [expire](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-retention-periods).
	ObjectLockRetainUntilDate *string `pulumi:"objectLockRetainUntilDate"`
	// Server-side encryption of the object in S3. Valid values are "`AES256`" and "`aws:kms`".
	ServerSideEncryption *string `pulumi:"serverSideEncryption"`
	// Path to a file that will be read and uploaded as raw bytes for the object content.
	Source pulumi.AssetOrArchive `pulumi:"source"`
	// Triggers updates like `etag` but useful to address `etag` encryption limitations.
	SourceHash *string `pulumi:"sourceHash"`
	// [Storage Class](https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html#AmazonS3-PutObject-request-header-StorageClass) for the object. Defaults to "`STANDARD`".
	StorageClass *string `pulumi:"storageClass"`
	// Map of tags to assign to the object. If configured with a provider `defaultTags` configuration block present, tags with matching keys will overwrite those defined at the provider-level.
	Tags map[string]string `pulumi:"tags"`
	// Map of tags assigned to the resource, including those inherited from the provider `defaultTags` configuration block.
	TagsAll map[string]string `pulumi:"tagsAll"`
	// Target URL for [website redirect](http://docs.aws.amazon.com/AmazonS3/latest/dev/how-to-page-redirect.html).
	WebsiteRedirect *string `pulumi:"websiteRedirect"`
}

// The set of arguments for constructing a BucketObject resource.
type BucketObjectArgs struct {
	// [Canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl) to apply. Valid values are `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`, `bucket-owner-read`, and `bucket-owner-full-control`. Defaults to `private`.
	Acl pulumi.PtrInputT[string]
	// Name of the bucket to put the file in. Alternatively, an [S3 access point](https://docs.aws.amazon.com/AmazonS3/latest/dev/using-access-points.html) ARN can be specified.
	Bucket pulumi.InputT[any]
	// Whether or not to use [Amazon S3 Bucket Keys](https://docs.aws.amazon.com/AmazonS3/latest/dev/bucket-key.html) for SSE-KMS.
	BucketKeyEnabled pulumi.PtrInputT[bool]
	// Caching behavior along the request/reply chain Read [w3c cacheControl](http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.9) for further details.
	CacheControl pulumi.PtrInputT[string]
	// Literal string value to use as the object content, which will be uploaded as UTF-8-encoded text.
	Content pulumi.PtrInputT[string]
	// Base64-encoded data that will be decoded and uploaded as raw bytes for the object content. This allows safely uploading non-UTF8 binary data, but is recommended only for small content such as the result of the `gzipbase64` function with small text strings. For larger objects, use `source` to stream the content from a disk file.
	ContentBase64 pulumi.PtrInputT[string]
	// Presentational information for the object. Read [w3c contentDisposition](http://www.w3.org/Protocols/rfc2616/rfc2616-sec19.html#sec19.5.1) for further information.
	ContentDisposition pulumi.PtrInputT[string]
	// Content encodings that have been applied to the object and thus what decoding mechanisms must be applied to obtain the media-type referenced by the Content-Type header field. Read [w3c content encoding](http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.11) for further information.
	ContentEncoding pulumi.PtrInputT[string]
	// Language the content is in e.g., en-US or en-GB.
	ContentLanguage pulumi.PtrInputT[string]
	// Standard MIME type describing the format of the object data, e.g., application/octet-stream. All Valid MIME Types are valid for this input.
	ContentType pulumi.PtrInputT[string]
	// Triggers updates when the value changes. This attribute is not compatible with KMS encryption, `kmsKeyId` or `serverSideEncryption = "aws:kms"` (see `sourceHash` instead).
	Etag pulumi.PtrInputT[string]
	// Whether to allow the object to be deleted by removing any legal hold on any object version. Default is `false`. This value should be set to `true` only if the bucket has S3 object lock enabled.
	ForceDestroy pulumi.PtrInputT[bool]
	// Name of the object once it is in the bucket.
	Key pulumi.PtrInputT[string]
	// ARN of the KMS Key to use for object encryption. If the S3 Bucket has server-side encryption enabled, that value will automatically be used. If referencing the `kms.Key` resource, use the `arn` attribute. If referencing the `kms.Alias` data source or resource, use the `targetKeyArn` attribute. The provider will only perform drift detection if a configuration value is provided.
	KmsKeyId pulumi.PtrInputT[string]
	// Map of keys/values to provision metadata (will be automatically prefixed by `x-amz-meta-`, note that only lowercase label are currently supported by the AWS Go API).
	Metadata pulumi.MapInputT[string]
	// [Legal hold](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-legal-holds) status that you want to apply to the specified object. Valid values are `ON` and `OFF`.
	ObjectLockLegalHoldStatus pulumi.PtrInputT[string]
	// Object lock [retention mode](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-retention-modes) that you want to apply to this object. Valid values are `GOVERNANCE` and `COMPLIANCE`.
	ObjectLockMode pulumi.PtrInputT[string]
	// Date and time, in [RFC3339 format](https://tools.ietf.org/html/rfc3339#section-5.8), when this object's object lock will [expire](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html#object-lock-retention-periods).
	ObjectLockRetainUntilDate pulumi.PtrInputT[string]
	// Server-side encryption of the object in S3. Valid values are "`AES256`" and "`aws:kms`".
	ServerSideEncryption pulumi.PtrInputT[string]
	// Path to a file that will be read and uploaded as raw bytes for the object content.
	Source pulumi.InputT[pulumi.AssetOrArchive]
	// Triggers updates like `etag` but useful to address `etag` encryption limitations.
	SourceHash pulumi.PtrInputT[string]
	// [Storage Class](https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html#AmazonS3-PutObject-request-header-StorageClass) for the object. Defaults to "`STANDARD`".
	StorageClass pulumi.PtrInputT[string]
	// Map of tags to assign to the object. If configured with a provider `defaultTags` configuration block present, tags with matching keys will overwrite those defined at the provider-level.
	Tags pulumi.MapInputT[string]
	// Map of tags assigned to the resource, including those inherited from the provider `defaultTags` configuration block.
	TagsAll pulumi.MapInputT[string]
	// Target URL for [website redirect](http://docs.aws.amazon.com/AmazonS3/latest/dev/how-to-page-redirect.html).
	WebsiteRedirect pulumi.PtrInputT[string]
}

func (BucketObjectArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*bucketObjectArgs)(nil)).Elem()
}
