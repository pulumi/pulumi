package s3

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Bucket struct {
	pulumi.CustomResourceState

	// Sets the accelerate configuration of an existing bucket. Can be `Enabled` or `Suspended`.
	AccelerationStatus pulumi.OutputT[string] `pulumi:"accelerationStatus"`
	// The [canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl) to apply. Valid values are `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`, and `log-delivery-write`. Defaults to `private`.  Conflicts with `grant`.
	Acl pulumi.StringPtrOutput `pulumi:"acl"`
	// The ARN of the bucket. Will be of format `arn:aws:s3:::bucketname`.
	Arn pulumi.OutputT[string] `pulumi:"arn"`
	// The name of the bucket. If omitted, this provider will assign a random, unique name. Must be lowercase and less than or equal to 63 characters in length. A full list of bucket naming rules [may be found here](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html).
	Bucket pulumi.OutputT[string] `pulumi:"bucket"`
	// The bucket domain name. Will be of format `bucketname.s3.amazonaws.com`.
	BucketDomainName pulumi.OutputT[string] `pulumi:"bucketDomainName"`
	// Creates a unique bucket name beginning with the specified prefix. Conflicts with `bucket`. Must be lowercase and less than or equal to 37 characters in length. A full list of bucket naming rules [may be found here](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html).
	BucketPrefix pulumi.StringPtrOutput `pulumi:"bucketPrefix"`
	// The bucket region-specific domain name. The bucket domain name including the region name, please refer [here](https://docs.aws.amazon.com/general/latest/gr/rande.html#s3_region) for format. Note: The AWS CloudFront allows specifying S3 region-specific endpoint when creating S3 origin, it will prevent [redirect issues](https://forums.aws.amazon.com/thread.jspa?threadID=216814) from CloudFront to S3 Origin URL.
	BucketRegionalDomainName pulumi.OutputT[string] `pulumi:"bucketRegionalDomainName"`
	// A rule of [Cross-Origin Resource Sharing](https://docs.aws.amazon.com/AmazonS3/latest/dev/cors.html) (documented below).
	CorsRules pulumi.ArrayOutputT[BucketCorsRule] `pulumi:"corsRules"`
	// A boolean that indicates all objects (including any [locked objects](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html)) should be deleted from the bucket so that the bucket can be destroyed without error. These objects are *not* recoverable.
	ForceDestroy pulumi.PtrOutputT[bool] `pulumi:"forceDestroy"`
	// An [ACL policy grant](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#sample-acl) (documented below). Conflicts with `acl`.
	Grants pulumi.ArrayOutputT[BucketGrant] `pulumi:"grants"`
	// The [Route 53 Hosted Zone ID](https://docs.aws.amazon.com/general/latest/gr/rande.html#s3_website_region_endpoints) for this bucket's region.
	HostedZoneId pulumi.OutputT[string] `pulumi:"hostedZoneId"`
	// A configuration of [object lifecycle management](http://docs.aws.amazon.com/AmazonS3/latest/dev/object-lifecycle-mgmt.html) (documented below).
	LifecycleRules pulumi.ArrayOutputT[BucketLifecycleRule] `pulumi:"lifecycleRules"`
	// A settings of [bucket logging](https://docs.aws.amazon.com/AmazonS3/latest/UG/ManagingBucketLogging.html) (documented below).
	Loggings pulumi.ArrayOutputT[BucketLogging] `pulumi:"loggings"`
	// A configuration of [S3 object locking](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock.html) (documented below)
	ObjectLockConfiguration pulumi.PtrOutputT[BucketObjectLockConfiguration] `pulumi:"objectLockConfiguration"`
	// A valid [bucket policy](https://docs.aws.amazon.com/AmazonS3/latest/dev/example-bucket-policies.html) JSON document. Note that if the policy document is not specific enough (but still valid), this provider may view the policy as constantly changing in a `pulumi preview`. In this case, please make sure you use the verbose/specific version of the policy.
	Policy pulumi.StringPtrOutput `pulumi:"policy"`
	// The AWS region this bucket resides in.
	Region pulumi.OutputT[string] `pulumi:"region"`
	// A configuration of [replication configuration](http://docs.aws.amazon.com/AmazonS3/latest/dev/crr.html) (documented below).
	ReplicationConfiguration pulumi.PtrOutputT[BucketReplicationConfiguration] `pulumi:"replicationConfiguration"`
	// Specifies who should bear the cost of Amazon S3 data transfer.
	// Can be either `BucketOwner` or `Requester`. By default, the owner of the S3 bucket would incur
	// the costs of any data transfer. See [Requester Pays Buckets](http://docs.aws.amazon.com/AmazonS3/latest/dev/RequesterPaysBuckets.html)
	// developer guide for more information.
	RequestPayer pulumi.OutputT[string] `pulumi:"requestPayer"`
	// A configuration of [server-side encryption configuration](http://docs.aws.amazon.com/AmazonS3/latest/dev/bucket-encryption.html) (documented below)
	ServerSideEncryptionConfiguration pulumi.OutputT[BucketServerSideEncryptionConfiguration] `pulumi:"serverSideEncryptionConfiguration"`
	// A map of tags to assign to the bucket. If configured with a provider `defaultTags` configuration block present, tags with matching keys will overwrite those defined at the provider-level.
	Tags pulumi.MapOutputT[string] `pulumi:"tags"`
	// A map of tags assigned to the resource, including those inherited from the provider `defaultTags` configuration block.
	TagsAll pulumi.MapOutputT[string] `pulumi:"tagsAll"`
	// A state of [versioning](https://docs.aws.amazon.com/AmazonS3/latest/dev/Versioning.html) (documented below)
	Versioning pulumi.OutputT[BucketVersioning] `pulumi:"versioning"`
	// A website object (documented below).
	Website pulumi.OutputT[BucketWebsite] `pulumi:"website"`
	// The domain of the website endpoint, if the bucket is configured with a website. If not, this will be an empty string. This is used to create Route 53 alias records.
	WebsiteDomain pulumi.OutputT[string] `pulumi:"websiteDomain"`
	// The website endpoint, if the bucket is configured with a website. If not, this will be an empty string.
	WebsiteEndpoint pulumi.OutputT[string] `pulumi:"websiteEndpoint"`
}

func NewBucket(ctx *pulumi.Context,
	name string, args *BucketArgs, opts ...pulumi.ResourceOption,
) (*Bucket, error) {
	if args == nil {
		args = &BucketArgs{}
	}

	var resource Bucket
	err := ctx.RegisterResource("aws:s3/bucket:Bucket", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type bucketArgs struct {
	// Sets the accelerate configuration of an existing bucket. Can be `Enabled` or `Suspended`.
	AccelerationStatus *string `pulumi:"accelerationStatus"`
	// The [canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl) to apply. Valid values are `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`, and `log-delivery-write`. Defaults to `private`.  Conflicts with `grant`.
	Acl *string `pulumi:"acl"`
	// The ARN of the bucket. Will be of format `arn:aws:s3:::bucketname`.
	Arn *string `pulumi:"arn"`
	// The name of the bucket. If omitted, this provider will assign a random, unique name. Must be lowercase and less than or equal to 63 characters in length. A full list of bucket naming rules [may be found here](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html).
	Bucket *string `pulumi:"bucket"`
	// Creates a unique bucket name beginning with the specified prefix. Conflicts with `bucket`. Must be lowercase and less than or equal to 37 characters in length. A full list of bucket naming rules [may be found here](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html).
	BucketPrefix *string `pulumi:"bucketPrefix"`
	// A rule of [Cross-Origin Resource Sharing](https://docs.aws.amazon.com/AmazonS3/latest/dev/cors.html) (documented below).
	CorsRules []BucketCorsRule `pulumi:"corsRules"`
	// A boolean that indicates all objects (including any [locked objects](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html)) should be deleted from the bucket so that the bucket can be destroyed without error. These objects are *not* recoverable.
	ForceDestroy *bool `pulumi:"forceDestroy"`
	// An [ACL policy grant](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#sample-acl) (documented below). Conflicts with `acl`.
	Grants []BucketGrant `pulumi:"grants"`
	// The [Route 53 Hosted Zone ID](https://docs.aws.amazon.com/general/latest/gr/rande.html#s3_website_region_endpoints) for this bucket's region.
	HostedZoneId *string `pulumi:"hostedZoneId"`
	// A configuration of [object lifecycle management](http://docs.aws.amazon.com/AmazonS3/latest/dev/object-lifecycle-mgmt.html) (documented below).
	LifecycleRules []BucketLifecycleRule `pulumi:"lifecycleRules"`
	// A settings of [bucket logging](https://docs.aws.amazon.com/AmazonS3/latest/UG/ManagingBucketLogging.html) (documented below).
	Loggings []BucketLogging `pulumi:"loggings"`
	// A configuration of [S3 object locking](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock.html) (documented below)
	ObjectLockConfiguration *BucketObjectLockConfiguration `pulumi:"objectLockConfiguration"`
	// A valid [bucket policy](https://docs.aws.amazon.com/AmazonS3/latest/dev/example-bucket-policies.html) JSON document. Note that if the policy document is not specific enough (but still valid), this provider may view the policy as constantly changing in a `pulumi preview`. In this case, please make sure you use the verbose/specific version of the policy.
	Policy interface{} `pulumi:"policy"`
	// A configuration of [replication configuration](http://docs.aws.amazon.com/AmazonS3/latest/dev/crr.html) (documented below).
	ReplicationConfiguration *BucketReplicationConfiguration `pulumi:"replicationConfiguration"`
	// Specifies who should bear the cost of Amazon S3 data transfer.
	// Can be either `BucketOwner` or `Requester`. By default, the owner of the S3 bucket would incur
	// the costs of any data transfer. See [Requester Pays Buckets](http://docs.aws.amazon.com/AmazonS3/latest/dev/RequesterPaysBuckets.html)
	// developer guide for more information.
	RequestPayer *string `pulumi:"requestPayer"`
	// A configuration of [server-side encryption configuration](http://docs.aws.amazon.com/AmazonS3/latest/dev/bucket-encryption.html) (documented below)
	ServerSideEncryptionConfiguration *BucketServerSideEncryptionConfiguration `pulumi:"serverSideEncryptionConfiguration"`
	// A map of tags to assign to the bucket. If configured with a provider `defaultTags` configuration block present, tags with matching keys will overwrite those defined at the provider-level.
	Tags map[string]string `pulumi:"tags"`
	// A state of [versioning](https://docs.aws.amazon.com/AmazonS3/latest/dev/Versioning.html) (documented below)
	Versioning *BucketVersioning `pulumi:"versioning"`
	// A website object (documented below).
	Website *BucketWebsite `pulumi:"website"`
	// The domain of the website endpoint, if the bucket is configured with a website. If not, this will be an empty string. This is used to create Route 53 alias records.
	WebsiteDomain *string `pulumi:"websiteDomain"`
	// The website endpoint, if the bucket is configured with a website. If not, this will be an empty string.
	WebsiteEndpoint *string `pulumi:"websiteEndpoint"`
}

// The set of arguments for constructing a Bucket resource.
type BucketArgs struct {
	// Sets the accelerate configuration of an existing bucket. Can be `Enabled` or `Suspended`.
	AccelerationStatus pulumi.PtrInputT[string]
	// The [canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl) to apply. Valid values are `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`, and `log-delivery-write`. Defaults to `private`.  Conflicts with `grant`.
	Acl pulumi.PtrInputT[string]
	// The ARN of the bucket. Will be of format `arn:aws:s3:::bucketname`.
	Arn pulumi.PtrInputT[string]
	// The name of the bucket. If omitted, this provider will assign a random, unique name. Must be lowercase and less than or equal to 63 characters in length. A full list of bucket naming rules [may be found here](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html).
	Bucket pulumi.PtrInputT[string]
	// Creates a unique bucket name beginning with the specified prefix. Conflicts with `bucket`. Must be lowercase and less than or equal to 37 characters in length. A full list of bucket naming rules [may be found here](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html).
	BucketPrefix pulumi.PtrInputT[string]
	// A rule of [Cross-Origin Resource Sharing](https://docs.aws.amazon.com/AmazonS3/latest/dev/cors.html) (documented below).
	CorsRules pulumi.ArrayInputT[BucketCorsRule]
	// A boolean that indicates all objects (including any [locked objects](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock-overview.html)) should be deleted from the bucket so that the bucket can be destroyed without error. These objects are *not* recoverable.
	ForceDestroy pulumi.PtrInputT[bool]
	// An [ACL policy grant](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#sample-acl) (documented below). Conflicts with `acl`.
	Grants pulumi.ArrayInputT[BucketGrant]
	// The [Route 53 Hosted Zone ID](https://docs.aws.amazon.com/general/latest/gr/rande.html#s3_website_region_endpoints) for this bucket's region.
	HostedZoneId pulumi.PtrInputT[string]
	// A configuration of [object lifecycle management](http://docs.aws.amazon.com/AmazonS3/latest/dev/object-lifecycle-mgmt.html) (documented below).
	LifecycleRules pulumi.ArrayInputT[BucketLifecycleRule]
	// A settings of [bucket logging](https://docs.aws.amazon.com/AmazonS3/latest/UG/ManagingBucketLogging.html) (documented below).
	Loggings pulumi.ArrayInputT[BucketLogging]
	// A configuration of [S3 object locking](https://docs.aws.amazon.com/AmazonS3/latest/dev/object-lock.html) (documented below)
	ObjectLockConfiguration pulumi.PtrInputT[BucketObjectLockConfiguration]
	// A valid [bucket policy](https://docs.aws.amazon.com/AmazonS3/latest/dev/example-bucket-policies.html) JSON document. Note that if the policy document is not specific enough (but still valid), this provider may view the policy as constantly changing in a `pulumi preview`. In this case, please make sure you use the verbose/specific version of the policy.
	Policy pulumi.InputT[any]
	// A configuration of [replication configuration](http://docs.aws.amazon.com/AmazonS3/latest/dev/crr.html) (documented below).
	ReplicationConfiguration pulumi.PtrInputT[BucketReplicationConfiguration]
	// Specifies who should bear the cost of Amazon S3 data transfer.
	// Can be either `BucketOwner` or `Requester`. By default, the owner of the S3 bucket would incur
	// the costs of any data transfer. See [Requester Pays Buckets](http://docs.aws.amazon.com/AmazonS3/latest/dev/RequesterPaysBuckets.html)
	// developer guide for more information.
	RequestPayer pulumi.PtrInputT[string]
	// A configuration of [server-side encryption configuration](http://docs.aws.amazon.com/AmazonS3/latest/dev/bucket-encryption.html) (documented below)
	ServerSideEncryptionConfiguration pulumi.PtrInputT[BucketServerSideEncryptionConfiguration]
	// A map of tags to assign to the bucket. If configured with a provider `defaultTags` configuration block present, tags with matching keys will overwrite those defined at the provider-level.
	Tags pulumi.MapInputT[string]
	// A state of [versioning](https://docs.aws.amazon.com/AmazonS3/latest/dev/Versioning.html) (documented below)
	Versioning pulumi.PtrInputT[BucketVersioning]
	// A website object (documented below).
	Website pulumi.InputT[BucketWebsite]
	// The domain of the website endpoint, if the bucket is configured with a website. If not, this will be an empty string. This is used to create Route 53 alias records.
	WebsiteDomain pulumi.PtrInputT[string]
	// The website endpoint, if the bucket is configured with a website. If not, this will be an empty string.
	WebsiteEndpoint pulumi.PtrInputT[string]
}

func (BucketArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*bucketArgs)(nil)).Elem()
}

type BucketCorsRule struct {
	// Specifies which headers are allowed.
	AllowedHeaders pulumi.ArrayInputT[string] `pulumi:"allowedHeaders"`
	// Specifies which methods are allowed. Can be `GET`, `PUT`, `POST`, `DELETE` or `HEAD`.
	AllowedMethods pulumi.ArrayInputT[string] `pulumi:"allowedMethods"`
	// Specifies which origins are allowed.
	AllowedOrigins pulumi.ArrayInputT[string] `pulumi:"allowedOrigins"`
	// Specifies expose header in the response.
	ExposeHeaders pulumi.ArrayInputT[string] `pulumi:"exposeHeaders"`
	// Specifies time in seconds that browser can cache the response for a preflight request.
	MaxAgeSeconds pulumi.InputT[int] `pulumi:"maxAgeSeconds"`
}

type BucketWebsite struct {
	// An absolute path to the document to return in case of a 4XX error.
	// ErrorDocument *string `pulumi:"errorDocument"`
	// Amazon S3 returns this index document when requests are made to the root domain or any of the subfolders.
	IndexDocument string `pulumi:"indexDocument"`
	// A hostname to redirect all website requests for this bucket to. Hostname can optionally be prefixed with a protocol (`http://` or `https://`) to use when redirecting requests. The default is the protocol that is used in the original request.
	// RedirectAllRequestsTo *string `pulumi:"redirectAllRequestsTo"`
	// A json array containing [routing rules](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-s3-websiteconfiguration-routingrules.html)
	// describing redirect behavior and when redirects are applied.
	// RoutingRules interface{} `pulumi:"routingRules"`
}

type BucketWebsiteInput interface {
	pulumi.Input

	ToBucketWebsiteOutput() BucketWebsiteOutput
	ToBucketWebsiteOutputWithContext(context.Context) BucketWebsiteOutput
}

type BucketWebsiteOutput struct{ *pulumi.OutputState }

func (BucketWebsiteOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*BucketWebsite)(nil)).Elem()
}

func (o BucketWebsiteOutput) ToOutputT(context.Context) pulumi.OutputT[BucketWebsite] {
	return pulumi.OutputT[BucketWebsite]{OutputState: o.OutputState}
}

func (o BucketWebsiteOutput) ToBucketWebsiteOutput() BucketWebsiteOutput {
	return o
}

func (o BucketWebsiteOutput) ToBucketWebsiteOutputWithContext(ctx context.Context) BucketWebsiteOutput {
	return o
}

type BucketWebsiteArgs struct {
	// // An absolute path to the document to return in case of a 4XX error.
	// ErrorDocument pulumi.InputT[string] `pulumi:"errorDocument"`
	// Amazon S3 returns this index document when requests are made to the root domain or any of the subfolders.
	IndexDocument pulumi.InputT[string] `pulumi:"indexDocument"`
	// // A hostname to redirect all website requests for this bucket to. Hostname can optionally be prefixed with a protocol (`http://` or `https://`) to use when redirecting requests. The default is the protocol that is used in the original request.
	// RedirectAllRequestsTo pulumi.InputT[string] `pulumi:"redirectAllRequestsTo"`
	// // A json array containing [routing rules](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-s3-websiteconfiguration-routingrules.html)
	// // describing redirect behavior and when redirects are applied.
	// RoutingRules pulumi.InputT[interface{}] `pulumi:"routingRules"`
}

func (BucketWebsiteArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*BucketWebsite)(nil)).Elem()
}

func (i BucketWebsiteArgs) ToOutputT(ctx context.Context) pulumi.OutputT[BucketWebsite] {
	return i.ToBucketWebsiteOutputWithContext(ctx).ToOutputT(ctx)
}

func (i BucketWebsiteArgs) ToBucketWebsiteOutput() BucketWebsiteOutput {
	return i.ToBucketWebsiteOutputWithContext(context.Background())
}

func (i BucketWebsiteArgs) ToBucketWebsiteOutputWithContext(ctx context.Context) BucketWebsiteOutput {
	return pulumi.ToOutputWithContext(ctx, i).(BucketWebsiteOutput)
}

type BucketGrant struct {
	// Canonical user id to grant for. Used only when `type` is `CanonicalUser`.
	Id pulumi.InputT[string] `pulumi:"id"`
	// List of permissions to apply for grantee. Valid values are `READ`, `WRITE`, `READ_ACP`, `WRITE_ACP`, `FULL_CONTROL`.
	Permissions pulumi.ArrayInputT[string] `pulumi:"permissions"`
	// - Type of grantee to apply for. Valid values are `CanonicalUser` and `Group`. `AmazonCustomerByEmail` is not supported.
	Type pulumi.InputT[string] `pulumi:"type"`
	// Uri address to grant for. Used only when `type` is `Group`.
	Uri pulumi.InputT[string] `pulumi:"uri"`
}

type BucketLogging struct {
	// The name of the bucket that will receive the log objects.
	TargetBucket pulumi.InputT[string] `pulumi:"targetBucket"`
	// To specify a key prefix for log objects.
	TargetPrefix pulumi.InputT[string] `pulumi:"targetPrefix"`
}

type BucketLifecycleRule struct {
	// Specifies the number of days after initiating a multipart upload when the multipart upload must be completed.
	AbortIncompleteMultipartUploadDays pulumi.InputT[int] `pulumi:"abortIncompleteMultipartUploadDays"`
	// Specifies lifecycle rule status.
	Enabled pulumi.InputT[bool] `pulumi:"enabled"`
	// Specifies a period in the object's expire (documented below).
	Expiration pulumi.InputT[BucketLifecycleRuleExpiration] `pulumi:"expiration"`
	// Unique identifier for the rule. Must be less than or equal to 255 characters in length.
	Id pulumi.InputT[string] `pulumi:"id"`
	// Specifies when noncurrent object versions expire (documented below).
	NoncurrentVersionExpiration pulumi.InputT[BucketLifecycleRuleNoncurrentVersionExpiration] `pulumi:"noncurrentVersionExpiration"`
	// Specifies when noncurrent object versions transitions (documented below).
	NoncurrentVersionTransitions pulumi.ArrayInputT[BucketLifecycleRuleNoncurrentVersionTransition] `pulumi:"noncurrentVersionTransitions"`
	// Object key prefix identifying one or more objects to which the rule applies.
	Prefix pulumi.InputT[string] `pulumi:"prefix"`
	// Specifies object tags key and value.
	Tags pulumi.MapInputT[string] `pulumi:"tags"`
	// Specifies a period in the object's transitions (documented below).
	Transitions []BucketLifecycleRuleTransition `pulumi:"transitions"`
}

type BucketLifecycleRuleExpiration struct {
	// Specifies the date after which you want the corresponding action to take effect.
	Date pulumi.InputT[string] `pulumi:"date"`
	// Specifies the number of days after object creation when the specific rule action takes effect.
	Days pulumi.InputT[int] `pulumi:"days"`
	// On a versioned bucket (versioning-enabled or versioning-suspended bucket), you can add this element in the lifecycle configuration to direct Amazon S3 to delete expired object delete markers. This cannot be specified with Days or Date in a Lifecycle Expiration Policy.
	ExpiredObjectDeleteMarker pulumi.InputT[bool] `pulumi:"expiredObjectDeleteMarker"`
}

type BucketLifecycleRuleNoncurrentVersionExpiration struct {
	// Specifies the number of days noncurrent object versions expire.
	Days pulumi.InputT[int] `pulumi:"days"`
}

type BucketLifecycleRuleNoncurrentVersionTransition struct {
	// Specifies the number of days noncurrent object versions transition.
	Days pulumi.InputT[int] `pulumi:"days"`
	// Specifies the Amazon S3 [storage class](https://docs.aws.amazon.com/AmazonS3/latest/API/API_Transition.html#AmazonS3-Type-Transition-StorageClass) to which you want the object to transition.
	StorageClass pulumi.InputT[string] `pulumi:"storageClass"`
}

type BucketLifecycleRuleTransition struct {
	// Specifies the date after which you want the corresponding action to take effect.
	Date pulumi.InputT[string] `pulumi:"date"`
	// Specifies the number of days after object creation when the specific rule action takes effect.
	Days pulumi.InputT[int] `pulumi:"days"`
	// Specifies the Amazon S3 [storage class](https://docs.aws.amazon.com/AmazonS3/latest/API/API_Transition.html#AmazonS3-Type-Transition-StorageClass) to which you want the object to transition.
	StorageClass pulumi.InputT[string] `pulumi:"storageClass"`
}

type BucketVersioning struct {
	// Enable versioning. Once you version-enable a bucket, it can never return to an unversioned state. You can, however, suspend versioning on that bucket.
	Enabled pulumi.InputT[bool] `pulumi:"enabled"`
	// Enable MFA delete for either `Change the versioning state of your bucket` or `Permanently delete an object version`. Default is `false`. This cannot be used to toggle this setting but is available to allow managed buckets to reflect the state in AWS
	MfaDelete pulumi.InputT[bool] `pulumi:"mfaDelete"`
}

type BucketObjectLockConfiguration struct {
	// Indicates whether this bucket has an Object Lock configuration enabled. Valid value is `Enabled`.
	ObjectLockEnabled pulumi.InputT[string] `pulumi:"objectLockEnabled"`
	// The Object Lock rule in place for this bucket.
	Rule pulumi.InputT[BucketObjectLockConfigurationRule] `pulumi:"rule"`
}

type BucketObjectLockConfigurationRule struct {
	// The default retention period that you want to apply to new objects placed in this bucket.
	DefaultRetention pulumi.InputT[BucketObjectLockConfigurationRuleDefaultRetention] `pulumi:"defaultRetention"`
}

type BucketObjectLockConfigurationRuleDefaultRetention struct {
	// The number of days that you want to specify for the default retention period.
	Days pulumi.InputT[int] `pulumi:"days"`
	// The default Object Lock retention mode you want to apply to new objects placed in this bucket. Valid values are `GOVERNANCE` and `COMPLIANCE`.
	Mode pulumi.InputT[string] `pulumi:"mode"`
	// The number of years that you want to specify for the default retention period.
	Years pulumi.InputT[int] `pulumi:"years"`
}

type BucketReplicationConfiguration struct {
	// The ARN of the IAM role for Amazon S3 to assume when replicating the objects.
	Role pulumi.InputT[string] `pulumi:"role"`
	// Specifies the rules managing the replication (documented below).
	Rules pulumi.ArrayInputT[BucketReplicationConfigurationRule] `pulumi:"rules"`
}

type BucketReplicationConfigurationRule struct {
	// Whether delete markers are replicated. The only valid value is `Enabled`. To disable, omit this argument. This argument is only valid with V2 replication configurations (i.e., when `filter` is used).
	DeleteMarkerReplicationStatus pulumi.InputT[string] `pulumi:"deleteMarkerReplicationStatus"`
	// Specifies the destination for the rule (documented below).
	Destination pulumi.InputT[BucketReplicationConfigurationRuleDestination] `pulumi:"destination"`
	// Filter that identifies subset of objects to which the replication rule applies (documented below).
	Filter pulumi.InputT[BucketReplicationConfigurationRuleFilter] `pulumi:"filter"`
	// Unique identifier for the rule. Must be less than or equal to 255 characters in length.
	Id pulumi.InputT[string] `pulumi:"id"`
	// Object keyname prefix identifying one or more objects to which the rule applies. Must be less than or equal to 1024 characters in length.
	Prefix pulumi.InputT[string] `pulumi:"prefix"`
	// The priority associated with the rule. Priority should only be set if `filter` is configured. If not provided, defaults to `0`. Priority must be unique between multiple rules.
	Priority pulumi.InputT[int] `pulumi:"priority"`
	// Specifies special object selection criteria (documented below).
	SourceSelectionCriteria pulumi.InputT[BucketReplicationConfigurationRuleSourceSelectionCriteria] `pulumi:"sourceSelectionCriteria"`
	// The status of the rule. Either `Enabled` or `Disabled`. The rule is ignored if status is not Enabled.
	Status pulumi.InputT[string] `pulumi:"status"`
}

type BucketReplicationConfigurationRuleSourceSelectionCriteria struct {
	// Match SSE-KMS encrypted objects (documented below). If specified, `replicaKmsKeyId`
	// in `destination` must be specified as well.
	SseKmsEncryptedObjects pulumi.InputT[BucketReplicationConfigurationRuleSourceSelectionCriteriaSseKmsEncryptedObjects] `pulumi:"sseKmsEncryptedObjects"`
}

type BucketReplicationConfigurationRuleSourceSelectionCriteriaSseKmsEncryptedObjects struct {
	// Boolean which indicates if this criteria is enabled.
	Enabled pulumi.InputT[bool] `pulumi:"enabled"`
}

type BucketReplicationConfigurationRuleFilter struct {
	// Object keyname prefix that identifies subset of objects to which the rule applies. Must be less than or equal to 1024 characters in length.
	Prefix pulumi.InputT[string] `pulumi:"prefix"`
	// A map of tags that identifies subset of objects to which the rule applies.
	// The rule applies only to objects having all the tags in its tagset.
	Tags pulumi.MapInputT[string] `pulumi:"tags"`
}

type BucketReplicationConfigurationRuleDestination struct {
	// Specifies the overrides to use for object owners on replication. Must be used in conjunction with `accountId` owner override configuration.
	AccessControlTranslation pulumi.InputT[BucketReplicationConfigurationRuleDestinationAccessControlTranslation] `pulumi:"accessControlTranslation"`
	// The Account ID to use for overriding the object owner on replication. Must be used in conjunction with `accessControlTranslation` override configuration.
	AccountId pulumi.InputT[string] `pulumi:"accountId"`
	// The ARN of the S3 bucket where you want Amazon S3 to store replicas of the object identified by the rule.
	Bucket pulumi.InputT[string] `pulumi:"bucket"`
	// Enables replication metrics (required for S3 RTC) (documented below).
	Metrics pulumi.InputT[BucketReplicationConfigurationRuleDestinationMetrics] `pulumi:"metrics"`
	// Destination KMS encryption key ARN for SSE-KMS replication. Must be used in conjunction with
	// `sseKmsEncryptedObjects` source selection criteria.
	ReplicaKmsKeyId pulumi.InputT[string] `pulumi:"replicaKmsKeyId"`
	// Enables S3 Replication Time Control (S3 RTC) (documented below).
	ReplicationTime pulumi.InputT[BucketReplicationConfigurationRuleDestinationReplicationTime] `pulumi:"replicationTime"`
	// The [storage class](https://docs.aws.amazon.com/AmazonS3/latest/API/API_Destination.html#AmazonS3-Type-Destination-StorageClass) used to store the object. By default, Amazon S3 uses the storage class of the source object to create the object replica.
	StorageClass pulumi.InputT[string] `pulumi:"storageClass"`
}

type BucketReplicationConfigurationRuleDestinationReplicationTime struct {
	// Threshold within which objects are to be replicated. The only valid value is `15`.
	Minutes pulumi.InputT[int] `pulumi:"minutes"`
	// The status of RTC. Either `Enabled` or `Disabled`.
	Status pulumi.InputT[string] `pulumi:"status"`
}

type BucketReplicationConfigurationRuleDestinationMetrics struct {
	// Threshold within which objects are to be replicated. The only valid value is `15`.
	Minutes pulumi.InputT[int] `pulumi:"minutes"`
	// The status of replication metrics. Either `Enabled` or `Disabled`.
	Status pulumi.InputT[string] `pulumi:"status"`
}

type BucketReplicationConfigurationRuleDestinationAccessControlTranslation struct {
	// The override value for the owner on replicated objects. Currently only `Destination` is supported.
	Owner pulumi.InputT[string] `pulumi:"owner"`
}

type BucketServerSideEncryptionConfiguration struct {
	// A single object for server-side encryption by default configuration. (documented below)
	Rule pulumi.InputT[BucketServerSideEncryptionConfigurationRule] `pulumi:"rule"`
}

type BucketServerSideEncryptionConfigurationRule struct {
	// A single object for setting server-side encryption by default. (documented below)
	ApplyServerSideEncryptionByDefault pulumi.InputT[BucketServerSideEncryptionConfigurationRuleApplyServerSideEncryptionByDefault] `pulumi:"applyServerSideEncryptionByDefault"`
	// Whether or not to use [Amazon S3 Bucket Keys](https://docs.aws.amazon.com/AmazonS3/latest/dev/bucket-key.html) for SSE-KMS.
	BucketKeyEnabled pulumi.InputT[bool] `pulumi:"bucketKeyEnabled"`
}

type BucketServerSideEncryptionConfigurationRuleApplyServerSideEncryptionByDefault struct {
	// The AWS KMS master key ID used for the SSE-KMS encryption. This can only be used when you set the value of `sseAlgorithm` as `aws:kms`. The default `aws/s3` AWS KMS master key is used if this element is absent while the `sseAlgorithm` is `aws:kms`.
	KmsMasterKeyId pulumi.InputT[string] `pulumi:"kmsMasterKeyId"`
	// The server-side encryption algorithm to use. Valid values are `AES256` and `aws:kms`
	SseAlgorithm pulumi.InputT[string] `pulumi:"sseAlgorithm"`
}

func init() {
	pulumi.RegisterInputType(reflect.TypeOf((*BucketWebsiteInput)(nil)).Elem(), BucketWebsiteArgs{})
	pulumi.RegisterOutputType(BucketWebsiteOutput{})
}
