// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';
import {InternetGateway} from './internetGateway';
import {RouteTable} from './routeTable';
import {VPCGatewayAttachment} from './vpcGatewayAttachment';
import * as cloudformation from '../cloudformation';

// A route in a route table within a VPC.
// @name: aws/ec2/route
// @website: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-route.html
export class Route extends cloudformation.Resource {
    constructor(ctx: mu.Context, args: RouteArgs) {
        super(ctx, {
            resource: "AWS::EC2::Route",
            dependsOn: [
                "vpcGatewayAttachment",
            ],
            properties: args,
        });
    }
}

export interface RouteArgs {
    // The CIDR address block used for the destination match.  For example, `0.0.0.0/0`.  Routing decisions are based
    // on the most specific match.
    readonly destinationCidrBlock: string;
    // The route table where the route will be added. 
    readonly routeTable: RouteTable;
    // The Internet gateway that is attached to your VPC.  For route entries that specify a gateway, you must also
    // specify a dependency on the gateway attachment resource (`vpcGatewayAttachment`).
    readonly internetGateway: InternetGateway;
    // The gateway attachment resource that attached the specified gateway to the VPC.
    readonly vpcGatewayAttachment: VPCGatewayAttachment;
}

