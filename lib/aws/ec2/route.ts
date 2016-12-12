// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';
import * as aws from 'aws';

// A route in a route table within a VPC.
// @name: aws/ec2/route
// @website: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-route.html
export default class Route extends aws.cloudformation.Resource {
    constructor(ctx: mu.Context, args: RouteArgs) {
        super(ctx, {
            resource: "AWS::EC2::Route",
            dependsOn: [
                "vpcGatewayAttachment",
            ],
            properties: {
                destinationCidrBlock: args.destinationCidrBlock,
                routeTableId: args.routeTable,
                gatewayId: args.internetGateway,
            },
        });
    }
}

export interface RouteArgs {
    // The CIDR address block used for the destination match.  For example, `0.0.0.0/0`.  Routing decisions are based
    // on the most specific match.
    readonly destinationCidrBlock: string;
    // The route table where the route will be added. 
    readonly routeTable: aws.ec2.routeTable;
    // The Internet gateway that is attached to your VPC.  For route entries that specify a gateway, you must also
    // specify a dependency on the gateway attachment resource (`vpcGatewayAttachment`).
    readonly internetGateway: aws.ec2.internetGateway;
    // The gateway attachment resource that attached the specified gateway to the VPC.
    readonly vpcGatewayAttachment: aws.ec2.vpcGatewayAttachment;
}

