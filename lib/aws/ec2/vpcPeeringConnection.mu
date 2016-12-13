// Copyright 2016 Marapongo, Inc. All rights reserved.

module "aws/ec2"
import "aws/cloudformation"

// A VPC peering connection enables a network connection between two virtual private clouds (VPCs) so that you can route
// traffic between them by means of a private IP addresses.
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-vpcpeeringconnection.html
service VPCPeeringConnection {
    ctor() {
        cloudformation.ExpandTags(this.properties)
        resource cloudformation.Resource {
            resource = "AWS::EC2::VPCPeeringConnection" 
            properties = this.properties
        }
    }

    properties: cloudformation.TagSchema {
        // The VPC with which you are creating the peering connection.
        readonly peerVpc: VPC
        // The VPC that is requesting a peering connection.
        readonly vpc: VPC
    }
}

