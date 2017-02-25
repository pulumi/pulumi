// Copyright 2016 Pulumi, Inc. All rights reserved.

module mu/clouds/aws
import aws/ec2

// A base Mu cluster running in AWS, ready to host stacks.
service Cluster {
    new() {
        // First set up a VPC with a single subnet.
        var cidr: "10.0.0.0/16"
        vpc := new ec2.VPC {
            name: context.cluster.name + "-VPC"
            cidrBlock: cidr
        }
        subnet := new ec2.Subnet {
            name: context.cluster.name + "-Subnet"
            vpc: vpc
            cidrBlock: cidr
        }

        // Now create an Internet-facing gateway to expose this cluster's subnet to Internet traffic.
        gateway := new ec2.InternetGateway {
            name: context.cluster.name + "-InternetGateway"
        }
        attachment := new ec2.VPCGatewayAttachment {
            internetGateway: gateway
            vpc: vpc
        }
        routes := new ec2.RouteTable {
            name: context.cluster.name + "-RouteTable"
            vpc: vpc
        }
        route := new ec2.Route {
            destinationCidrBlock: "0.0.0.0/0"
            internetGateway: gateway
            vpcGatewayAttachment: attachment
            routeTable: routes
        }

        // Finally, create a sole security group to use for everything by default.
        group := new ec2.SecurityGroup {
            name: context.cluster.name + "-SecurityGroup"
            vpc: vpc
            groupDescription: "The primary cluster's security group."
        }
    }
}

