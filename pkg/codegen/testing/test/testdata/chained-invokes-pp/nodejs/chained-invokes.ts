import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const vpcId = aws.ec2.getVpc({
    "default": true,
}).then(invoke => invoke.id);
const subnetIds = aws.ec2.getSubnetIds({
    vpcId: vpcId,
}).then(invoke => invoke.ids);
