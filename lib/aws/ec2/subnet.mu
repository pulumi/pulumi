// Copyright 2016 Marapongo, Inc. All rights reserved.

module "aws/ec2"
import "aws/cloudformation"

// A subnet in an existing VPC.
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-subnet.html
service Subnet {
    ctor() {
        cloudformation.ExpandTags(this.properties)
        new cloudformation.Resource {
            resource = "AWS::EC2::Subnet"
            properties = this.properties
        }
    }

    properties: cloudformation.TagSchema {
        // The CIDR block that you want the subnet to cover (for example, `"10.0.0.0/24"`).
        readonly cidrBlock: string
        // The VPC on which you want to create the subnet.
        readonly vpc: VPC
        // The availability zone in which you want the subnet.  By default, AWS selects a zone for you.
        optional readonly availabilityZone: string
        // Indicates whether instances that are launched in this subnet receive a public IP address.  By default, `false`.
        optional mapPublicIpOnLaunch: bool
    }
}

