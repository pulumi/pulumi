// Copyright 2016 Marapongo, Inc. All rights reserved.
"use strict";
const mu = require("mu");
const aws = require("mu-aws");
// A base Mu cluster, ready to host stacks.
class Cluster extends mu.Stack {
    constructor(args) {
        super();
        // TODO: support anonymous clusters (e.g., for local testing).
        // TODO: load cluster targeting information from other places:
        //      1) workspace settings (e.g., map keyed by cluster name).
        //      2) general configuration system (e.g., defaults).
        switch (args.arch.cloud) {
            case "aws":
                this.createAWSCloudResources(args);
                break;
            default:
                throw new Error(`Unrecognized/unimplemented cloud target: ${args.arch.cloud}`);
        }
    }
    // This function creates all of the basic resources necessary for an AWS cluster ready to host Mu stacks.
    createAWSCloudResources(args) {
        // First set up a VPC with a single subnet.
        let cidr = "10.0.0.0/16";
        let vpc = new aws.ec2.VPC({ name: `${args.name}-VPC`, cidrBlock: cidr });
        let subnet = new aws.ec2.Subnet({ name: `${args.name}-Subnet`, vpc: vpc, cidrBlock: cidr });
        // Now create an Internet-facing gateway to expose this cluster's subnet to Internet traffic.
        let internetGateway = new aws.ec2.InternetGateway({ name: `${args.name}-InternetGateway` });
        let vpcGatewayAttachment = new aws.ec2.VPCGatewayAttachment({ internetGateway: internetGateway, vpc: vpc });
        let routeTable = new aws.ec2.RouteTable({ name: `${args.name}-RouteTable`, vpc: vpc });
        let route = new aws.ec2.Route({
            destinationCidrBlock: "0.0.0.0/0",
            internetGateway: internetGateway,
            vpcGatewayAttachment: vpcGatewayAttachment,
            routeTable: routeTable,
        });
        // Finally, create a sole security group to use for everything by default.
        let securityGroup = new aws.ec2.SecurityGroup({
            name: `${args.name}-SecurityGroup`,
            vpc: vpc,
            groupDescription: "The primary cluster's security group.",
        });
    }
}
Object.defineProperty(exports, "__esModule", { value: true });
exports.default = Cluster;
//# sourceMappingURL=cluster.js.map