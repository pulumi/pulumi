// Copyright 2016 Pulumi, Inc. All rights reserved.

import * as aws from "@coconut/aws";
import * as amimap from "./amimap";

let instanceType = "t2.micro";
let sshLocation = "0.0.0.0";
let sshLocationCIDR = sshLocation + "/0";
let region = aws.config.requireRegion();

let securityGroup = new aws.ec2.SecurityGroup("group", {
    groupDescription: "Enable SSH access and WWW",
    securityGroupIngress: [
        {
            ipProtocol: "tcp",
            fromPort: 22,
            toPort: 22,
            cidrIp: sshLocationCIDR,
        },
        {
            ipProtocol: "tcp",
            fromPort: 80,
            toPort: 80,
            cidrIp: sshLocationCIDR,
        },
    ]
});

let arch = amimap.awsInstanceType2Arch[instanceType].Arch;
let image = amimap.awsRegionArch2AMI[region][arch];
let instance = new aws.ec2.Instance("instance", {
    instanceType: instanceType,
    securityGroups: [ securityGroup ],
    imageId: image,
});

