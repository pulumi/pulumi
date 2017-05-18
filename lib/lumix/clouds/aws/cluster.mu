// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

