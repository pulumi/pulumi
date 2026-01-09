import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const config = new pulumi.Config();
// The tag of the VPC
const vpcTag = config.get("vpcTag");
// The id of a VPC to use instead of creating a new one
const vpcId = config.get("vpcId");
// The list of subnets to use
const subnets = config.getObject<Array<string>>("subnets");
// Additional tags to add to the VPC
const moreTags = config.getObject<Record<string, string>>("moreTags");
// The userdata to use for the instances
const userdata = config.getObject<{content?: string, path?: string}>("userdata");
// A complex object
const complexUserdata = config.getObject<Array<{content?: string, path?: string}>>("complexUserdata");
const main = new aws.ec2.Vpc("main", {
    cidrBlock: "10.100.0.0/16",
    tags: {
        Name: vpcTag,
    },
});
