// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package sns

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// An Amazon Simple Notification Service (Amazon SNS) topic subscription.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-sns-subscription.html.
type Subscription struct {
	idl.NamedResource
	// A name for the topic.  If you don't specify a name, a unique physical ID will be generated.
	Topic *Topic `lumi:"topic,replaces"`
	// The subscription's protocol.
	Protocol Protocol `lumi:"protocol,replaces"`
	// The subscription's endpoint (format depends on the protocol).
	Endpoint string `lumi:"endpoint,replaces"`
}

// The protocols supported by the Amazon Simple Notification Service (Amazon SNS).
type Protocol string

const (
	HTTSubscription         Protocol = "http"        // delivery of JSON-encoded message via HTTP POST.
	HTTPSSubscription       Protocol = "https"       // delivery of JSON-encoded message via HTTPS POST.
	EmailSubscription       Protocol = "email"       // delivery of message via SMTP.
	EmailJSONSubscription   Protocol = "email-json"  // delivery of JSON-encoded message via SMTP.
	SMSSubscription         Protocol = "sms"         // delivery of message via SMS.
	SQSSubscription         Protocol = "sqs"         // delivery of JSON-encoded message to an Amazon SQS queue.
	ApplicationSubscription Protocol = "application" // delivery of JSON-encoded message to a mobile app or device.
	LambdaSubscription      Protocol = "lambda"      // delivery of JSON-encoded message to an AWS Lambda function.
)
