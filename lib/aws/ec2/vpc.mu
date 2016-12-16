// Copyright 2016 Marapongo, Inc. All rights reserved.

module aws/ec2
import aws/cloudformation

// A Virtual Private Cloud (VPC) with a specified CIDR block.
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-vpc.html
service VPC {
    new() {
        cloudformation.ExpandTags(this.properties)
        cf := new cloudformation.Resource {
            resource: "AWS::EC2::VPC"
            properties: this.properties
        }
    }

    properties: cloudformation.TagSchema {
        // The CIDR block you want the VPC to cover.  For example, "10.0.0.0/16".
        readonly cidrBlock: string
        // The allowed tenancy of instances launched into the VPC.  "default" indicates that instances can be launched with
        // any tenancy, while "dedicated" indicates that any instance launched into the VPC automatically has dedicated
        // tenancy, unless you launch it with the default tenancy.
        optional readonly instanceTenancy: "default" | "dedicated"
        // Specifies whether DNS resolution is supported for the VPC.  If true, the Amazon DNS server resolves DNS hostnames
        // for your instances to their corresponding IP addresses; otherwise, it does not.  By default, the value is true. 
        optional enableDnsSupport: bool
        // Specifies whether the instances launched in the VPC get DNS hostnames.  If this attribute is true, instances in
        // the VPC get DNS hostnames; otherwise, they do not.  You can only set enableDnsHostnames to true if you also set
        // the enableDnsSupport property to true.  By default, the value is set to false.
        optional enableDnsHostnames: bool
    }
}

