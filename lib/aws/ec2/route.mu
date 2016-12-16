// Copyright 2016 Marapongo, Inc. All rights reserved.

module aws/ec2
import aws/cloudformation

// A route in a route table within a VPC.
// @website: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-route.html
service Route {
    new() {
        cf := new cloudformation.Resource {
            resource: "AWS::EC2::Route"
            dependsOn: [ vpcGatewayAttachment ]
            properties: this.properties
        }
    }

    properties {
        // The CIDR address block used for the destination match.  For example, `0.0.0.0/0`.  Routing decisions are based
        // on the most specific match.
        readonly destinationCidrBlock: string
        // The route table where the route will be added. 
        readonly routeTable: routeTable
        // The Internet gateway that is attached to your VPC.  For route entries that specify a gateway, you must also
        // specify a dependency on the gateway attachment resource (`vpcGatewayAttachment`).
        readonly internetGateway: internetGateway
        // The gateway attachment resource that attached the specified gateway to the VPC.
        readonly vpcGatewayAttachment: vpcGatewayAttachment
    }
}

