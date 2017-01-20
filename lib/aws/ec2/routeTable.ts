// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';
import {VPC} from './vpc';
import * as cloudformation from '../cloudformation';

// A new route table within your VPC.  After creating a route table, you can add routes and associate the table with a
// subnet.
// @name: aws/ec2/routeTable
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-route-table.html
export class RouteTable extends cloudformation.Resource {
    constructor(args: RouteTableArgs) {
        cloudformation.expandTags(args);
        super({
            resource: "AWS::EC2::RouteTable",
            properties: args,
        });
    }
}

export interface RouteTableArgs extends cloudformation.TagArgs {
    // The VPC where the route table will be created.
    readonly vpc: VPC;
}

