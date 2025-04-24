import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const webSecurityGroup = new aws.ec2.SecurityGroup("webSecurityGroup", {vpcId: aws.ec2.getVpc({
    "default": true,
}).then(invoke => invoke.id)});
