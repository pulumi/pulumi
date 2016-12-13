// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';
import * as aws from 'mu-aws';

// A base Mu cluster, ready to host stacks.
// @name: mu/cluster
export default class Cluster extends mu.Stack {
    constructor(ctx: mu.Context) {
        super(ctx);
        switch (ctx.arch.cloud) {
            case mu.clouds.AWS:
                this.createAWSCloudResources(ctx);
                break;
            default:
                throw new Error(`Unrecognized cloud target: ctx.arch.cloud`);
        }
    }

    // This function creates all of the basic resources necessary for an AWS cluster ready to host Mu stacks.
    private createAWSCloudResources(ctx: mu.Context): void {
        // First set up a VPC with a single subnet.
        let cidr = "10.0.0.0/16";
        let vpc = new aws.ec2.VPC(ctx, { name: `${ctx.cluster.name}-VPC`, cidrBlock: cidr });
        let subnet = new aws.ec2.Subnet(ctx, { name: `${ctx.cluster.name}-Subnet`,  vpc: vpc,  cidrBlock: cidr });

        // Now create an Internet-facing gateway to expose this cluster's subnet to Internet traffic.
        let internetGateway = new aws.ec2.InternetGateway(ctx, { name: `${ctx.cluster.name}-InternetGateway` });
        let vpcGatewayAttachment = new aws.ec2.VPCGatewayAttachment(
            ctx, { internetGateway: internetGateway, vpc: vpc });
        let routeTable = new aws.ec2.RouteTable(ctx, { name: `${ctx.cluster.name}-RouteTable`, vpc: vpc });
        let route = new aws.ec2.Route(ctx, {
            destinationCidrBlock: "0.0.0.0/0",
            internetGateway: internetGateway,
            vpcGatewayAttachment: vpcGatewayAttachment,
            routeTable: routeTable,
        });

        // Finally, create a sole security group to use for everything by default.
        let securityGroup = new aws.ec2.SecurityGroup(ctx, {
            name: `${ctx.cluster.name}-SecurityGroup`,
            vpc: vpc,
            groupDescription: "The primary cluster's security group.",
        });
    }
}

