// Copyright 2017 Pulumi, Inc. All rights reserved.

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
export type ARN = string;

// Region contains all the valid AWS regions in a convenient union type.
export type Region =
    "us-east-1"      | // US East (N. Virginia)
    "us-east-2"      | // US East (Ohio)
    "us-west-1"      | // US West (N. California)
    "us-west-2"      | // US West (Oregon)
    "ca-central"     | // Canada (Central)
    "ap-south-1"     | // Asia Pacific (Mumbai)
    "ap-northeast-1" | // Asia Pacific (Tokyo)
    "ap-northeast-2" | // Asia Pacific (Seoul)
    "ap-southeast-1" | // Asia Pacific (Singapore)
    "ap-southeast-2" | // Asia Pacific (Sydney)
    "eu-central-1"   | // EU (Frankfurt)
    "eu-west-1"      | // EU (Ireland)
    "eu-west-2"      | // EU (London)
    "sa-east-1"      ; // South America (Sao Paulo)

