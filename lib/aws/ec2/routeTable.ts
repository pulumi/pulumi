// Copyright 2017 Pulumi, Inc. All rights reserved.

import {VPC} from './vpc';
import * as cloudformation from '../cloudformation';

// A new route table within your VPC.  After creating a route table, you can add routes and associate the table with a
// subnet.
// @name: aws/ec2/routeTable
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-route-table.html
export class RouteTable extends cloudformation.Resource {
    constructor(name: string, args: RouteTableArgs) {
        super({
            name: name,
            resource: "AWS::EC2::RouteTable",
            properties: args,
        });
    }
}

export interface RouteTableArgs extends cloudformation.TagArgs {
    // The VPC where the route table will be created.
    readonly vpc: VPC;
}

