import * as aws from "@coconut/aws";

let vpc = new aws.ec2.VPC({ cidrBlock: "10.0.0.0/16" });

