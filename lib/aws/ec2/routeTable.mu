// Copyright 2016 Marapongo, Inc. All rights reserved.

module aws/ec2
import aws/cloudformation

// A new route table within your VPC.  After creating a route table, you can add routes and associate the table with a
// subnet.
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-route-table.html
service RouteTable {
    new() {
        cloudformation.ExpandTags(this.properties)
        cf := new cloudformation.Resource {
            resource: "AWS::EC2::RouteTable"
            properties: this.properties
        }
    }

    properties: cloudformation.TagSchema {
        // The VPC where the route table will be created.
        readonly vpc: VPC
    }
}

