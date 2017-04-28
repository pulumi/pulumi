// Copyright 2017 Pulumi, Inc. All rights reserved.

package lambda

import (
	"github.com/pulumi/coconut/pkg/resource/idl"

	aws "github.com/pulumi/coconut/lib/aws/idl"
	"github.com/pulumi/coconut/lib/aws/idl/ec2"
	"github.com/pulumi/coconut/lib/aws/idl/iam"
	"github.com/pulumi/coconut/lib/aws/idl/kms"
)

// The Function resource creates an AWS Lambda function that can run code in response to events.
type Function struct {
	idl.NamedResource

	// code is the source code of your Lambda function.  This supports all the usual Coconut asset schemes, in addition
	// to Amazon Simple Storage Service (S3) bucket locations, indicating with a URI scheme of s3//<bucket>/<object>.
	Code *idl.Asset `coco:"code"`
	// handler is the name of the function (within your source code) that Lambda calls to start running your code.
	Handler string `coco:"handler"`
	// role is the AWS Identity and Access Management (IAM) execution role that Lambda assumes when it runs your code
	// to access AWS services.
	Role *iam.Role `coco:"role"`
	// runtime is the runtime environment for the Lambda function that you are uploading.
	Runtime Runtime `coco:"runtime"`
	// functionName is a name for the function.  If you don't specify a name, a unique physical ID is used instead.
	FunctionName *string `coco:"functionName,optional"`
	// deadLetterConfig configures how Lambda handles events that it can't process.  If you don't specify a Dead Letter
	// Queue (DLQ) configuration, Lambda discards events after the maximum number of retries.
	DeadLetterConfig *DeadLetterConfig `coco:"deadLetterConfig,optional"`
	// description is an optional description of the function.
	Description *string `coco:"description,optional"`
	// environment contains key-value pairs that Lambda caches and makes available for your Lambda functions.  Use
	// environment variables to apply configuration changes, such as test and production environment configurations,
	// without changing your Lambda function source code.
	Environment *Environment `coco:"environment,optional"`
	// kmsKey is a AWS Key Management Service (AMS KMS) key that Lambda uses to encrypt and decrypt environment
	// variables.
	KMSKey *kms.Key `coco:"kmsKey,optional"`
	// memorySize is the amount of memory, in MB, that is allocated to your Lambda function.  Lambda uses this value to
	// proportionally allocate the amount of CPU power.  Your function use case determines your CPU and memory
	// requirements.  For example, a database operation might need less memory than an image processing function.  You
	// must specify a value that is greater than or equal to `128` and it must be a multiple of `64`.  You cannot
	// specify a size larger than `1536`.  The default value is `128` MB.
	MemorySize *float64 `coco:"memorySize,optional"`
	// timeout is the function execution time (in seconds) after which Lambda terminates the function.  Because the
	// execution time affects cost, set this value based on the function's expected execution time.  By default, timeout
	// is set to `3` seconds.
	Timeout *float64 `coco:"timeout,optional"`
	// vpcConfig specifies a VPC configuration that Lambda uses to set up an elastic network interface (ENI).  The ENI
	// enables your function to connect to other resources in your VPC, but it doesn't provide public Internet access.
	// If your function requires Internet access (for example, to access AWS services that don't have VPC endpoints),
	// configure a Network Address Translation (NAT) instance inside your VPC or use an Amazon Virtual Private Cloud
	// (Amazon VPC) NAT gateway.
	VPCConfig *VPCConfig `coco:"vpcConfig,optional"`

	// The ARN of the Lambda function, such as `arn:aws:lambda:us-west-2:123456789012:MyStack-AMILookUp-NT5EUXTNTXXD`.
	ARN aws.ARN `coco:"arn,out"`
}

// Runtime represents the legal runtime environments for Lambdas.
type Runtime string

const (
	NodeJSRuntime        Runtime = "nodejs"
	NodeJS4d3Runtime     Runtime = "nodejs4.3"
	NodeJS4d3EdgeRuntime Runtime = "nodejs4.3-edge"
	NodeJS6d10Runtime    Runtime = "nodejs6.10"
	Java8Runtime         Runtime = "java8"
	Python2d7Runtime     Runtime = "python2.7"
	DotnetCore1d0Runtime Runtime = "dotnetcore1.0"
)

// DeadLetterConfig is a property of an AWS Lambda Function resource that specifies a Dead Letter Queue (DLQ) that
// events are sent to when functions cannot be processed.  For example, you can send unprocessed events to an Amazon
// Simple Notification Service (Amazon SQS) topic, where you can take further action.
type DeadLetterConfig struct {
	// target is the target resource where Lambda delivers unprocessed events.  It may be an Amazon SNS topic or Amazon
	// Simple Queue Service (SQS) queue.  For the Lambda function-execution role, you must explicitly provide the
	// relevant permissions so that access to your DLQ resource is part of the execution role for your Lambda function.
	Target *idl.Resource `coco:"target"` // TODO: sns.Topic | sqs.Queue;
}

// Environment is a property of an AWS Lambda Function resource that specifies key-value pairs that the function can
// access so that you can apply configuration changes, such as test and production environment configurations, without
// changing the function code.
type Environment map[string]string

// VPCConfig is a property of an AWS Lambda Function resource that enables it to access resources in a VPC.  For more
// information, see http://docs.aws.amazon.com/lambda/latest/dg/vpc.html.
type VPCConfig struct {
	// securityGroups is a list of one or more security groups in the VPC that include the resources to which your
	// Lambda function requires access.
	SecurityGroups []*ec2.SecurityGroup `coco:"securityGroups"`
	// subnets is a list of one or more subnet IDs in the VPC that includes the resources to which your Lambda function
	// requires access.
	Subnets []*ec2.Subnet `coco:"subnets"`
}
