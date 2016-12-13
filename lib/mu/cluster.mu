// Copyright 2016 Marapongo, Inc. All rights reserved.

module "mu"
import "aws/ec2"

// A base Mu cluster, ready to host stacks.
service Cluster {
    ctor() {
        switch context.arch.cloud {
        case "aws":
            createAWSResources()
        default:
            panic("Unrecognized cloud target: %v", context.arch.cloud)
        }
    }

    macro createAWSResources() {
        // First set up a VPC with a single subnet.
        var cidr = "10.0.0.0/16"
        resource ec2.VPC {
            name = context.cluster.name + "-VPC"
            cidrBlock = cidr
        }
        resource ec2.Subnet {
            name = context.cluster.name + "-Subnet"
            vpc = VPC
            cidrBlock = cidr
        }
        // Now create an Internet-facing gateway to expose this cluster's subnet to Internet traffic.
        resource ec2.InternetGateway {
            name = context.cluster.name + "-InternetGateway"
        }
        resource ec2.VPCGatewayAttachment {
            internetGateway = internetGateway
            vpc = VPC
        }
        resource ec2.RouteTable {
            name = context.cluster.name + "-RouteTable"
            vpc = VPC
        }
        resource ec2.Route {
            destinationCidrBlock = "0.0.0.0/0"
            internetGateway = InternetGateway
            vpcGatewayAttachment = VPCGatewayAttachment
            routeTable = RouteTable
        }

        // Finally, create a sole security group to use for everything by default.
        resource ec2.SecurityGroup {
            name = context.cluster.name + "-SecurityGroup"
            vpc = VPC
            groupDescription = "The primary cluster's security group."
        }
    }
}

