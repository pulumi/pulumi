package s3point5

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Bucket struct {
	pulumi.CustomResourceState

	// Sets the accelerate configuration of an existing bucket. Can be `Enabled` or `Suspended`.
	AccelerationStatus pulumi.Promise[string] `pulumi:"accelerationStatus"`
	// The [canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl) to apply. Valid values are `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`, and `log-delivery-write`. Defaults to `private`.  Conflicts with `grant`.
	Acl pulumi.Promise[*string] `pulumi:"acl"`
	// The ARN of the bucket. Will be of format `arn:aws:s3:::bucketname`.
	Arn pulumi.Promise[string] `pulumi:"arn"`
	// The name of the bucket. If omitted, this provider will assign a random, unique name. Must be lowercase and less than or equal to 63 characters in length. A full list of bucket naming rules [may be found here](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html).
	Bucket pulumi.Promise[string] `pulumi:"bucket"`
	// The bucket domain name. Will be of format `bucketname.s3.amazonaws.com`.
	BucketDomainName pulumi.Promise[string] `pulumi:"bucketDomainName"`
	// Creates a unique bucket name beginning with the specified prefix. Conflicts with `bucket`. Must be lowercase and less than or equal to 37 characters in length. A full list of bucket naming rules [may be found here](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html).
	BucketPrefix pulumi.Promise[*string] `pulumi:"bucketPrefix"`
	// The bucket region-specific domain name. The bucket domain name including the region name, please refer [here](https://docs.aws.amazon.com/general/latest/gr/rande.html#s3_region) for format. Note: The AWS CloudFront allows specifying S3 region-specific endpoint when creating S3 origin, it will prevent [redirect issues](https://forums.aws.amazon.com/thread.jspa?threadID=216814) from CloudFront to S3 Origin URL.
	BucketRegionalDomainName pulumi.Promise[string] `pulumi:"bucketRegionalDomainName"`
	// A rule of [Cross-Origin Resource Sharing](https://docs.aws.amazon.com/AmazonS3/latest/dev/cors.html) (documented below).
	CorsRules pulumi.Promise[[]s3.BucketCorsRule] `pulumi:"corsRules"`
	// A boolean that indicates all objects (including any [locked objects](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html)) should be deleted from the bucket so that the bucket can be destroyed without error. These objects are *not* recoverable.
	ForceDestroy pulumi.Promise[*bool] `pulumi:"forceDestroy"`
	// An [ACL policy grant](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#sample-acl) (documented below). Conflicts with `acl`.
	Grants pulumi.Promise[[]s3.BucketGrant] `pulumi:"grants"`
	// The [Route 53 Hosted Zone ID](https://docs.aws.amazon.com/general/latest/gr/rande.html#s3_website_region_endpoints) for this bucket's region.
	HostedZoneId pulumi.Promise[string] `pulumi:"hostedZoneId"`
	// A configuration of [object lifecycle management](http://docs.aws.amazon.com/AmazonS3/latest/dev/object-lifecycle-mgmt.html) (documented below).
	LifecycleRules pulumi.Promise[[]s3.BucketLifecycleRule] `pulumi:"lifecycleRules"`
	// A settings of [bucket logging](https://docs.aws.amazon.com/AmazonS3/latest/UG/ManagingBucketLogging.html) (documented below).
	Loggings pulumi.Promise[[]s3.BucketLogging] `pulumi:"loggings"`
	// A configuration of [S3 object locking](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock.html) (documented below)
	ObjectLockConfiguration pulumi.Promise[*s3.BucketObjectLockConfiguration] `pulumi:"objectLockConfiguration"`
	// A valid [bucket policy](https://docs.aws.amazon.com/AmazonS3/latest/dev/example-bucket-policies.html) JSON document. Note that if the policy document is not specific enough (but still valid), this provider may view the policy as constantly changing in a `pulumi preview`. In this case, please make sure you use the verbose/specific version of the policy.
	Policy pulumi.Promise[*string] `pulumi:"policy"`
	// The AWS region this bucket resides in.
	Region pulumi.Promise[string] `pulumi:"region"`
	// A configuration of [replication configuration](http://docs.aws.amazon.com/AmazonS3/latest/dev/crr.html) (documented below).
	ReplicationConfiguration pulumi.Promise[*s3.BucketReplicationConfiguration] `pulumi:"replicationConfiguration"`
	// Specifies who should bear the cost of Amazon S3 data transfer.
	// Can be either `BucketOwner` or `Requester`. By default, the owner of the S3 bucket would incur
	// the costs of any data transfer. See [Requester Pays Buckets](http://docs.aws.amazon.com/AmazonS3/latest/dev/RequesterPaysBuckets.html)
	// developer guide for more information.
	RequestPayer pulumi.Promise[string] `pulumi:"requestPayer"`
	// A configuration of [server-side encryption configuration](http://docs.aws.amazon.com/AmazonS3/latest/dev/bucket-encryption.html) (documented below)
	ServerSideEncryptionConfiguration pulumi.Promise[*s3.BucketServerSideEncryptionConfiguration] `pulumi:"serverSideEncryptionConfiguration"`
	// A map of tags to assign to the bucket. If configured with a provider `defaultTags` configuration block present, tags with matching keys will overwrite those defined at the provider-level.
	Tags pulumi.Promise[map[string]string] `pulumi:"tags"`
	// A map of tags assigned to the resource, including those inherited from the provider `defaultTags` configuration block.
	TagsAll pulumi.Promise[map[string]string] `pulumi:"tagsAll"`
	// A state of [versioning](https://docs.aws.amazon.com/AmazonS3/latest/dev/Versioning.html) (documented below)
	Versioning pulumi.Promise[s3.BucketVersioning] `pulumi:"versioning"`
	// A website object (documented below).
	Website pulumi.Promise[*s3.BucketWebsite] `pulumi:"website"`
	// The domain of the website endpoint, if the bucket is configured with a website. If not, this will be an empty string. This is used to create Route 53 alias records.
	WebsiteDomain pulumi.Promise[string] `pulumi:"websiteDomain"`
	// The website endpoint, if the bucket is configured with a website. If not, this will be an empty string.
	WebsiteEndpoint pulumi.Promise[string] `pulumi:"websiteEndpoint"`
}
