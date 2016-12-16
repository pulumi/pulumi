// Copyright 2016 Marapongo, Inc. All rights reserved.

module aws/ec2
import aws/cloudformation

// Attaches a gateway to a VPC.
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-vpc-gateway-attachment.html
service VPCGatewayAttachment {
    new() {
        cf := new cloudformation.Resource {
            resource: "AWS::EC2::VPCGatewayAttachment"
            properties: this.properties
        }
    }

    properties {
        // The VPC to associate with this gateway.
        readonly vpc: VPC
        // The Internet gateway to attach to the VPC.
        readonly internetGateway: InternetGateway
    }
}

