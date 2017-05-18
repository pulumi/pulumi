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

package sqs

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The Queue resource creates an Amzon Simple Queue Service (Amazon SQS) queue.  For more information about creating
// FIFO (first-in-first-out) queues, see http://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/.
type Queue struct {
	idl.NamedResource
	// FIFOQueue indicates whether this queue is a FIFO queue.  The default value is `false`.
	FIFOQueue *bool `lumi:"fifoQueue,replaces,optional"`
	// queueName is a name for the queue.  To create a FIFO queue, the name of your FIFO queue must end with the `.fifo`
	// suffix.  If you don't specify a name, a unique physical ID will be generated and used.
	//
	// Important: If you specify a name, you cannot perform updates that require replacement of this resource.  You can
	// perform updates that require no or some interruption.  If you must replace the resource, specify a new name.
	QueueName *string `lumi:"queueName,replaces,optional"`
	// contentBasedDeduplication, for first-in-first-out (FIFO) queues, specifies whether to enable content-based
	// deduplication.  During the deduplication interval, Amazon SQS treats messages that are sent with identical
	// content as duplicates and delivers only one copy of the message.
	ContentBasedDeduplication *bool `lumi:"contentBasedDeduplication,optional"`
	// delaySeconds is the time in seconds that the delivery of all messages in the queue is delayed.  You can specify
	// an integer value of `0` to `900` (15 minutes).  The default value is `0`.
	DelaySeconds *float64 `lumi:"delaySeconds,optional"`
	// maximumMessageSize sets a limit of how many bytes that a message can contain before Amazon SQS rejects it.  You
	// can specify an integer value from `1024` bytes (1 KiB) to `262144` bytes (256 KiB).  The default value is
	// `262144` bytes (256 KiB).
	MaximumMessageSize *float64 `lumi:"maximumMessageSize,optional"`
	// messageRetentionPeriod is the number of seconds that Amazon SQS retains a message.  You can specify an integer
	// value from `60` seconds (1 minute) to `1209600` seconds (14 days).  The default value is `345600` (4 days).
	MessageRetentionPeriod *float64 `lumi:"messageRetentionPeriod,optional"`
	// receiveMessageWaitTimeSeconds specifies the duration, in seconds, that receiving a message waits until a message
	// is in the queue in order to include it in the response, as opposed to returning an empty response if a message is
	// not yet available.  You can specify an integer from `1` to `20`. The short polling is used as the default
	// or when you specify `0` for this property. For more information about SQS Long Polling, see
	// http://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-long-polling.html.
	ReceiveMessageWaitTimeSeconds *float64 `lumi:"receiveMessageWaitTimeSeconds,optional"`
	// redrivePolicy specifies an existing dead letter queue to receive messages after the source queue (this queue)
	// fails to process a message a specified number of times.
	RedrivePolicy *RedrivePolicy `lumi:"redrivePolicy,optional"`
	// visibilityTimeout specifies the length of time during which a message will be unavailable after a message is
	// delivered from the queue. This blocks other components from receiving the same message and gives the initial
	// component time to process and delete the message from the queue.
	//
	// Values must be from `0` to `43200` seconds (12 hours).  The default value if unspecified is `30` seconds.
	//
	// For more information about Amazon SQS queue visibility timeouts, see Visibility Timeouts in
	// http://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/AboutVT.html.
	VisibilityTimeout *float64 `lumi:"visibilityTimeout,optional"`
}

type RedrivePolicy struct {
	// deadLetterTarget is the dead letter queue to which the messages are sent after maxReceiveCount has been exceeded.
	DeadLetterTarget *Queue `lumi:"deadLetterTarget"`
	// maxReceiveCount is the number of times a message is delivered to the source queue before being sent to the dead
	// letter queue.
	MaxReceiveCount float64 `lumi:"maxReceiveCount"`
}
