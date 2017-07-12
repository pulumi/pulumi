// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cloudwatch

import (
	aws "github.com/pulumi/lumi/lib/aws/idl"
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// LogSubscriptionFilter is a CloudWatch Logs subscription filter.  For more information, see
// http://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/CreateSubscriptionFilter.html.
type LogSubscriptionFilter struct {
	idl.NamedResource
	// The name of the log group.
	LogGroupName string `lumi:"logGroupName,replaces"`
	// A filter pattern for subscribing to a filtered stream of log events.
	FilterPattern string `lumi:"filterPattern"`
	// The ARN of the destination to deliver matching log events to.
	DestinationArn string `lumi:"destinationArn,replaces"`
	// The ARN of an IAM role that grants CloudWatch Logs permissions to deliver ingested log events to the
	// destination stream. You don't need to provide the ARN when you are working with a logical destination for
	// cross-account delivery.
	RoleARN *aws.ARN `lumi:"roleArn,optional"`
	// The method used to distribute log data to the destination, when the destination is an Amazon Kinesis stream.
	// By default, log data is grouped by log stream. For a more even distribution, you can group log data randomly.
	Distribution *LogSubscriptionDistribution `lumi:"distribution,optional"`

	// The time the log group subscription gilter was created.
	CreationTime *float64 `lumi:"creationTime,out"`
}

type LogSubscriptionDistribution string

const (
	RandomDistribution      LogSubscriptionDistribution = "Random"
	ByLogStreamDistribution LogSubscriptionDistribution = "ByLogStream"
)
