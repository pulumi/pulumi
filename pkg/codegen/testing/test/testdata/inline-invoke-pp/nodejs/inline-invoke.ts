import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const webServer = new aws.ec2.Instance("webServer", {ami: aws.getAmi({
    filters: [{
        name: "name",
        values: ["amzn-ami-hvm-*-x86_64-ebs"],
    }],
    owners: ["137112412989"],
    mostRecent: true,
}).then(invoke => invoke.id)});
