import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const aws_vpc = new aws.ec2.Vpc("aws_vpc", {
    cidrBlock: "10.0.0.0/16",
    instanceTenancy: "default",
});
const privateS3VpcEndpoint = new aws.ec2.VpcEndpoint("privateS3VpcEndpoint", {
    vpcId: aws_vpc.id,
    serviceName: "com.amazonaws.us-west-2.s3",
});
const privateS3PrefixList = aws.ec2.getPrefixListOutput({
    prefixListId: privateS3VpcEndpoint.prefixListId,
});
const bar = new aws.ec2.NetworkAcl("bar", {vpcId: aws_vpc.id});
const privateS3NetworkAclRule = new aws.ec2.NetworkAclRule("privateS3NetworkAclRule", {
    networkAclId: bar.id,
    ruleNumber: 200,
    egress: false,
    protocol: "tcp",
    ruleAction: "allow",
    cidrBlock: privateS3PrefixList.cidrBlocks[0],
    fromPort: 443,
    toPort: 443,
});
// A contrived example to test that helper nested records ( `filters`
// below) generate correctly when using output-versioned function
// invoke forms.
const amis = aws.ec2.getAmiIdsOutput({
    owners: [bar.id],
    filters: [{
        name: bar.id,
        values: ["pulumi*"],
    }],
});
