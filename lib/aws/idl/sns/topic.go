// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	// The SNS subscriptions (endpoints) for this topic.
	Subscription *[]TopicSubscription `lumi:"subscription,optional"`
}

type TopicSubscription struct {
	// The subscription's protocol.
	Protocol TopicProtocol `lumi:"protocol"`
	// The subscription's endpoint (format depends on the protocol).
	Endpoint string `lumi:"endpoint"`
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
