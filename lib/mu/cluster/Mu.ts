import * as mu from 'mu';
import * as aws from 'aws';

// A base Mu cluster, ready to host stacks.
// @name: mu/cluster
export default class Cluster extends mu.Stack {
    constructor(ctx: mu.Context) {
        super(ctx);
        switch (ctx.Arch.Cloud) {
            case "aws":
                this.createAWSCloudResources(ctx);
            default:
                throw new Error(`Unrecognized cloud target: %v`, ctx.Arch.Cloud);
        }
    }

    // This function creates all of the basic resources necessary for an AWS cluster ready to host Mu stacks.
    private createAWSCloudResources(ctx: mu.Context): void {
        // First set up a VPC with a single subnet.
        let cidr = "10.0.0.0/16";
        let vpc = new aws.ec2.VPC({ name: `${ctx.Cluster.Name}-VPC`, cidrBlock: cidr });
        let subnet = new aws.ec2.Subnet({ name: `${ctx.Cluster.Name}-Subnet`,  vpc: vpc,  cidrBlock: cidr });

        // Now create an Internet-facing gateway to expose this cluster's subnet to Internet traffic.
        let internetGateway = new aws.ec2.InternetGateway({ name: `${ctx.Cluster.Name}-InternetGateway` });
        let vpcGatewayAttachment = new aws.ec2.VPCGatewayAttachment({ internetGateway: internetGateway, vpc: vpc });
        let routeTable = new aws.ec2.RouteTable({ name: `${Cluster.Name}-RouteTable`, vpc: vpc });
        let route = new aws.ec2.Route({
            destinationCidrBlock: "0.0.0.0/0",
            internetGateway: internetGateway,
            vpcGatewayAttachment: vpcGatewayAttachment,
            routeTable: routeTable,
        });

        // Finally, create a sole security group to use for everything by default.
        let securityGroup = new aws.ec2.SecurityGroup({
            name: `${Cluster.Name}-SecurityGroup`,
            vpc: vpc,
            groupDescription: "The primary cluster's security group.",
        });
    }
}

