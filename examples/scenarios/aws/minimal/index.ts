import * as aws from "@lumi/aws";

let vpc = new aws.ec2.VPC({ cidrBlock: "10.0.0.0/16" });

