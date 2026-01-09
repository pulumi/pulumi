import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const config = new pulumi.Config();
const instanceType = config.get("InstanceType") || "t3.micro";
const ec2Ami = aws.getAmi({
    filters: [{
        name: "name",
        values: ["amzn-ami-hvm-*-x86_64-ebs"],
    }],
    owners: ["137112412989"],
    mostRecent: true,
}).then(invoke => invoke.id);
const webSecGrp = new aws.ec2.SecurityGroup("WebSecGrp", {ingress: [{
    protocol: "tcp",
    fromPort: 80,
    toPort: 80,
    cidrBlocks: ["0.0.0.0/0"],
}]});
const webServer = new aws.ec2.Instance("WebServer", {
    instanceType: instanceType,
    ami: ec2Ami,
    userData: pulumi.interpolate`#!/bin/bash
echo 'Hello, World from ${webSecGrp.arn}!' > index.html
nohup python -m SimpleHTTPServer 80 &`,
    vpcSecurityGroupIds: [webSecGrp.id],
});
const usEast2Provider = new aws.Provider("UsEast2Provider", {region: "us-east-2"});
const myBucket = new aws.s3.Bucket("MyBucket", {}, {
    provider: usEast2Provider,
});
export const instanceId = webServer.id;
export const publicIp = webServer.publicIp;
export const publicHostName = webServer.publicDns;
