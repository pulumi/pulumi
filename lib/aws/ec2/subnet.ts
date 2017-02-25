// Copyright 2016 Pulumi, Inc. All rights reserved.

import {VPC} from './vpc';
import * as cloudformation from '../cloudformation';

// A subnet in an existing VPC.
// @name: aws/ec2/subnet
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-subnet.html
export class Subnet extends cloudformation.Resource {
    constructor(name: string, args: SubnetArgs) {
        super({
            name: name,
            resource: "AWS::EC2::Subnet",
            properties: args,
        });
    }
}

export interface SubnetArgs extends cloudformation.TagArgs {
    // The CIDR block that you want the subnet to cover (for example, `"10.0.0.0/24"`).
    readonly cidrBlock: string;
    // The VPC on which you want to create the subnet.
    readonly vpc: VPC;
    // The availability zone in which you want the subnet.  By default, AWS selects a zone for you.
    readonly availabilityZone?: string;
    // Indicates whether instances that are launched in this subnet receive a public IP address.  By default, `false`.
    mapPublicIpOnLaunch?: boolean;
}

