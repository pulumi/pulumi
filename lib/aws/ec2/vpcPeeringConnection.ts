// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from '@mu/mu';
import {VPC} from './vpc';
import * as cloudformation from '../cloudformation';

// A VPC peering connection enables a network connection between two virtual private clouds (VPCs) so that you can route
// traffic between them by means of a private IP addresses.
// @name: aws/ec2/vpcPeeringConnection
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-vpcpeeringconnection.html
export class VPCPeeringConnection extends cloudformation.Resource {
    constructor(name: string, args: VPCPeeringConnectionArgs) {
        super({
            name: name,
            resource: "AWS::EC2::VPCPeeringConnection",
            properties: args,
        });
    }
}

export interface VPCPeeringConnectionArgs extends cloudformation.TagArgs {
    // The VPC with which you are creating the peering connection.
    readonly peerVpc: VPC;
    // The VPC that is requesting a peering connection.
    readonly vpc: VPC;
}

