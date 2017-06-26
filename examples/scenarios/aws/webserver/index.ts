// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as aws from "@lumi/aws";

export let size: aws.ec2.InstanceType = "t2.micro";

let group = new aws.ec2.SecurityGroup("web-secgrp", {
    groupDescription: "Enable HTTP and SSH access",
    securityGroupIngress: [
        { ipProtocol: "tcp", fromPort: 80, toPort: 80, cidrIp: "0.0.0.0/0" },
        { ipProtocol: "tcp", fromPort: 22, toPort: 22, cidrIp: "0.0.0.0/0" },
    ],
});

let server = new aws.ec2.Instance("web-server-www", {
    instanceType: size,
    securityGroups: [ group ],
    imageId: aws.ec2.getLinuxAMI(size),
});

