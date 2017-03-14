// Copyright 2016 Pulumi, Inc. All rights reserved.

import * as aws from "@coconut/aws";
import * as amimap from "./amimap";

export let instanceType = "t2.micro"; // a configurable kind of instance.

let securityGroup = new aws.ec2.SecurityGroup("web-secgrp", {
    groupDescription: "Enable HTTP access",
    securityGroupIngress: [
        { ipProtocol: "tcp", fromPort: 80, toPort: 80, cidrIp: "0.0.0.0/0" },
    ]
});

let instance = new aws.ec2.Instance("web-server", {
    instanceType: instanceType,
    securityGroups: [ securityGroup ],
    imageId: amimap.getLinuxAMI(instanceType),
});

