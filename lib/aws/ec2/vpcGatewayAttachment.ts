// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';
import {InternetGateway} from './internetGateway';
import {VPC} from './vpc';
import * as cloudformation from '../cloudformation';

// Attaches a gateway to a VPC.
// @name: aws/ec2/vpcGatewayAttachment
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-vpc-gateway-attachment.html
export class VPCGatewayAttachment extends cloudformation.Resource {
    constructor(args: VPCGatewayAttachmentArgs) {
        super({
            resource: "AWS::EC2::VPCGatewayAttachment",
            properties: args,
        });
    }
}

export interface VPCGatewayAttachmentArgs {
    // The VPC to associate with this gateway.
    readonly vpc: VPC;
    // The Internet gateway to attach to the VPC.
    readonly internetGateway: InternetGateway;
}

