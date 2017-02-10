import * as mu from "mu";
import * as aws from "@mu/aws";

let vm = new aws.ec2.VPC({ cidrBlock: "10.0.0.0/16" });

