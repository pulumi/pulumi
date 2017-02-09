// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from '@mu/mu';
import * as cloudformation from '../cloudformation';

// An Internet gateway enables your instances to connect to the Internet through the Amazon EC2 edge network.
// @name: aws/ec2/internetGateway
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-internet-gateway.html 
export class InternetGateway extends cloudformation.Resource {
    constructor(args: InternetGatewayArgs) {
        cloudformation.expandTags(args);
        super({
            resource: "AWS::EC2::InternetGateway",
            properties: args,
        });
    }
}

export interface InternetGatewayArgs extends cloudformation.TagArgs {
}

