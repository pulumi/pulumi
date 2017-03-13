// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as cloudformation from "../cloudformation";

// An Amazon Simple Notification Service (Amazon SNS) topic.
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-sns-topic.html
export class Topic
        extends cloudformation.Resource
        implements TopicProperties {

    public readonly topicName?: string;
    public displayName?: string;
    public subscription?: TopicSubscription[];

    constructor(name: string, args: TopicProperties) {
        super({
            name: name,
            resource:  "AWS::EC2::SecurityGroup",
        });
        this.topicName = args.topicName;
        this.displayName = args.displayName;
        this.subscription = args.subscription;
    }
}

export interface TopicProperties {
    // A name for the topic.  If you don't specify a name, a unique physical ID will be generated.
    readonly topicName?: string;
    // A developer-defined string that can be used to identify this SNS topic.
    displayName?: string;
    // The SNS subscriptions (endpoints) for this topic.
    subscription?: TopicSubscription[];
}

export interface TopicSubscription {
    // The subscription's protocol.
    readonly protocol: TopicProtocol;
    // The subscription's endpoint (format depends on the protocol).
    readonly endpoint: string;
}

// The protocols supported by the Amazon Simple Notification Service (Amazon SNS).
export type TopicProtocol =
    "http"        | // delivery of JSON-encoded message via HTTP POST.
    "https"       | // delivery of JSON-encoded message via HTTPS POST.
    "email"       | // delivery of message via SMTP.
    "email-json"  | // delivery of JSON-encoded message via SMTP.
    "sms"         | // delivery of message via SMS.
    "sqs"         | // delivery of JSON-encoded message to an Amazon SQS queue.
    "application" | // delivery of JSON-encoded message to an EndpointArn for a mobile app and device.
    "lambda"      ; // delivery of JSON-encoded message to an AWS Lambda function.

