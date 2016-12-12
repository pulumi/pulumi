// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';
import * as aws from 'aws';

// An Internet gateway enables your instances to connect to the Internet through the Amazon EC2 edge network.
// @name: aws/ec2/internetGateway
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-internet-gateway.html 
export default class InternetGateway extends aws.cloudformation.Resource {
    constructor(ctx: mu.Context, args: InternetGatewayArgs) {
        super(ctx, {
            resource: "AWS::EC2::InternetGateway",
            properties: {
                tags: aws.tagsPlusName(args.tags, args.name),
            },
        });
    }
}

export interface InternetGatewayArgs {
    // An optional name for this Internet gateway.
    name?: string;
    // An arbitrary set of tags (key-value pairs) for this Internet gateway.
    tags?: aws.Tag[];
}

