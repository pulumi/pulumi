// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cloudwatch

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// LogGroup is a CloudWatch Logs log group.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-logs-loggroup.html.
type LogGroup struct {
	idl.NamedResource
	// The name of the log group.
	LogGroupName *string `lumi:"logGroupName,optional,replaces"`
	// The number of days log events are kept in CloudWatch Logs. When a log event expires, CloudWatch Logs automatically deletes it.
	RetentionInDays *float64 `lumi:"retentionInDays,optional"`
}
