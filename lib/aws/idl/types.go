// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package aws

// Amazon Resource Names (ARNs) uniquely identify AWS resources.  An ARN is required when you need to specify a
// resource unambiguously across all of AWS, such as in IAM policies, Amazon Relational Database Service (Amazon RDS)
// tags, and API calls.
//
// Here are some example ARNs:
//
//     * Elastic Beanstalk application version:
//       arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/My App/MyEnvironment
//     * IAM user name:
//       arn:aws:iam::123456789012:user/David
//     * Amazon RDS instance used for tagging:
//       arn:aws:rds:eu-west-1:123456789012:db:mysql-db
//     * Object in an Amazon S3 bucket:
//       arn:aws:s3:::my_corporate_bucket/exampleobject.png
//
// The following are the general formats for ARNs; the specific components and values depend on the AWS service:
//
//      arn:partition:service:region:account-id:resource
//      arn:partition:service:region:account-id:resourcetype/resource
//      arn:partition:service:region:account-id:resourcetype:resource
//
// The component parts are:
//
//    * `partition`: The partition that the resource is in.  For standard AWS regions, the partition is `aws`.  If you
//          have resources in other partitions, the partition is `aws-partitionname`.  For example, the partition for
//          resources in the China (Beijing) region is `aws-cn`.
//    * `service`: The service namespace that identifies the AWS product (for example, Amazon S3, IAM, or Amazon RDS).
//          For a list of namespaces, see
//          http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html#genref-aws-service-namespaces.
//    * `region`: The region the resource resides in.  Note that the ARNs for some resources do not require a region,
//          so this component might be omitted.
//    * `account`: The ID of the AWS account that owns the resource, without the hyphens. For example, 123456789012.
//          Note that the ARNs for some resources don't require an account number, so this component might be omitted.
//    * `resource`, `resourcetype/resource`, or `resourcetype:resource`: The content of this part of the ARN varies by
//          service. It often includes an indicator of the type of resource—for example, an IAM user or Amazon RDS
//          database -- followed by a slash (/) or a colon (:), followed by the resource name itself.  Some services
//          allows paths for resource names, as described in
//          http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html#arns-paths.
//
// For more information on ARNs, please see http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html.
type ARN string

// Region contains all the valid AWS regions in a convenient union type.
type Region string

const (
	USEast1Region      Region = "us-east-1"      // US East (N. Virginia)
	USEast2Region      Region = "us-east-2"      // US East (Ohio)
	USWest1Region      Region = "us-west-1"      // US West (N. California)
	USWest2Region      Region = "us-west-2"      // US West (Oregon)
	CACentralRegion    Region = "ca-central"     // Canada (Central)
	APSouth1Region     Region = "ap-south-1"     // Asia Pacific (Mumbai)
	APNortheast1Region Region = "ap-northeast-1" // Asia Pacific (Tokyo)
	APNortheast2Region Region = "ap-northeast-2" // Asia Pacific (Seoul)
	APSoutheast1Region Region = "ap-southeast-1" // Asia Pacific (Singapore)
	APSouthEast2Region Region = "ap-southeast-2" // Asia Pacific (Sydney)
	EUCentral1Region   Region = "eu-central-1"   // EU (Frankfurt)
	EUWest1Region      Region = "eu-west-1"      // EU (Ireland)
	EUWest2Region      Region = "eu-west-2"      // EU (London)
	SAEast1Region      Region = "sa-east-1"      // South America (Sao Paulo)
)
