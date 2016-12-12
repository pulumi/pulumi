// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';
import * as aws from 'aws';

// A subnet in an existing VPC.
// @name: aws/ec2/subnet
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-subnet.html
export class Subnet extends aws.cloudformation.Resource {
    constructor(ctx: mu.Context, args: SubnetArgs) {
        super(ctx, {
            resource: "AWS::EC2::Subnet",
            properties: {
                cidrBlock: args.cidrBlock,
                vpcId: args.vpc,
                availabilityZone: args.availabilityZone,
                mapPublicIpOnLaunch: args.mapPublicIpOnLaunch,
                tags: aws.tagsPlusName(args.tags, args.name),
            },
        });
    }
}

export interface SubnetArgs {
    // The CIDR block that you want the subnet to cover (for example, `"10.0.0.0/24"`).
    readonly cidrBlock: string;
    // The VPC on which you want to create the subnet.
    readonly vpc: aws.ec2.VPC;
    // The availability zone in which you want the subnet.  By default, AWS selects a zone for you.
    readonly availabilityZone?: string;
    // Indicates whether instances that are launched in this subnet receive a public IP address.  By default, `false`.
    mapPublicIpOnLaunch?: boolean;
    // An optional name for this resource.
    name?: string;
    // An arbitrary set of tags (key-value pairs) for this resource.
    tags?: aws.Tag[];
}

