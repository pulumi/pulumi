// Copyright 2017 Pulumi, Inc. All rights reserved.

package sns

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// An Amazon Simple Notification Service (Amazon SNS) topic.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-sns-topic.html.
type Topic struct {
	idl.NamedResource
	// A name for the topic.  If you don't specify a name, a unique physical ID will be generated.
	TopicName *string `coco:"topicName,replaces,optional"`
	// A developer-defined string that can be used to identify this SNS topic.
	DisplayName *string `coco:"displayName,optional"`
	// The SNS subscriptions (endpoints) for this topic.
	Subscription *[]TopicSubscription `coco:"subscription,optional"`
}

type TopicSubscription struct {
	// The subscription's protocol.
	Protocol TopicProtocol `coco:"protocol"`
	// The subscription's endpoint (format depends on the protocol).
	Endpoint string `coco:"endpoint"`
}

// The protocols supported by the Amazon Simple Notification Service (Amazon SNS).
type TopicProtocol string

const (
	HTTPTopic        TopicProtocol = "http"        // delivery of JSON-encoded message via HTTP POST.
	HTTPSTopic       TopicProtocol = "https"       // delivery of JSON-encoded message via HTTPS POST.
	EmailTopic       TopicProtocol = "email"       // delivery of message via SMTP.
	EmailJSONTopic   TopicProtocol = "email-json"  // delivery of JSON-encoded message via SMTP.
	SMSTopic         TopicProtocol = "sms"         // delivery of message via SMS.
	SQSTopic         TopicProtocol = "sqs"         // delivery of JSON-encoded message to an Amazon SQS queue.
	ApplicationTopic TopicProtocol = "application" // delivery of JSON-encoded message to a mobile app or device.
	LambdaTopic      TopicProtocol = "lambda"      // delivery of JSON-encoded message to an AWS Lambda function.
)
