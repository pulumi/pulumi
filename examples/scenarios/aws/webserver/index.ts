// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as aws from "@coconut/aws";

export let instanceType: aws.ec2.InstanceType = "t2.micro";

let securityGroup = new aws.ec2.SecurityGroup("web-secgrp", {
    groupDescription: "Enable HTTP access",
    securityGroupIngress: [
        { ipProtocol: "tcp", fromPort: 80, toPort: 80, cidrIp: "0.0.0.0/0" },
    ]
});

let instance = new aws.ec2.Instance("web-server", {
    instanceType: instanceType,
    securityGroups: [ securityGroup ],
    imageId: aws.ec2.getLinuxAMI(instanceType),
});

