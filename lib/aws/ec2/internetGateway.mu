// Copyright 2016 Marapongo, Inc. All rights reserved.

module "aws/ec2"
import "aws/cloudformation"

// An Internet gateway enables your instances to connect to the Internet through the Amazon EC2 edge network.
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-internet-gateway.html 
service InternetGateway {
    ctor() {
        cloudformation.ExpandTags(this.properties)
        new cloudformation.Resource {
            resource = "AWS::EC2::InternetGateway"
            properties = this.properties
        }
    }

    properties: cloudFormation.TagSchema {
    }
}

