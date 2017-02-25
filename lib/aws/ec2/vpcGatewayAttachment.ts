// Copyright 2016 Pulumi, Inc. All rights reserved.

import {InternetGateway} from './internetGateway';
import {VPC} from './vpc';
import * as cloudformation from '../cloudformation';

// Attaches a gateway to a VPC.
// @name: aws/ec2/vpcGatewayAttachment
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-vpc-gateway-attachment.html
export class VPCGatewayAttachment extends cloudformation.Resource {
    constructor(name: string, args: VPCGatewayAttachmentArgs) {
        super({
            name: name,
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

