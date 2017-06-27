// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package sns

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// An Amazon Simple Notification Service (Amazon SNS) topic.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-sns-topic.html.
type Topic struct {
	idl.NamedResource
	// A name for the topic.  If you don't specify a name, a unique physical ID will be generated.
	TopicName *string `lumi:"topicName,replaces,optional"`
	// A developer-defined string that can be used to identify this SNS topic.
	DisplayName *string `lumi:"displayName,optional"`
}
