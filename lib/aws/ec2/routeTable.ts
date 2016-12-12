// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';
import * as aws from 'aws';

// A new route table within your VPC.  After creating a route table, you can add routes and associate the table with a
// subnet.
// @name: aws/ec2/routeTable
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-route-table.html
export class RouteTable extends aws.cloudformation.Resource {
    constructor(ctx: mu.Context, args: RouteTableArgs) {
        super(ctx, {
            resource: "AWS::EC2::RouteTable",
            properties: {
                vpcId: args.vpc,
                tags: aws.tagsPlusName(args.tags, args.name),
            },
        });
    }
}

export interface RouteTableArgs {
    // The VPC where the route table will be created.
    readonly vpc: aws.ec2.VPC;
    // An optional name for this route table resource.
    name?: string;
    // An arbitrary set of tags (key-value pairs) for this route table.
    tags?: aws.Tag[];
}

