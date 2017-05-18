// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as aws from "@lumi/aws";

let group = new aws.ec2.SecurityGroup("web-secgrp", {
    groupDescription: "Enable HTTP access",
    securityGroupIngress: [
        { ipProtocol: "tcp", fromPort: 80, toPort: 80, cidrIp: "0.0.0.0/0" },
    ]
});

export class Server {
    public readonly instance: aws.ec2.Instance;

    constructor(name: string, size: aws.ec2.InstanceType) {
        this.instance = new aws.ec2.Instance("web-server-" + name, {
            instanceType: size,
            securityGroups: [ group ],
            imageId: aws.ec2.getLinuxAMI(size),
        });
    }
}

export class Micro extends Server {
    constructor(name: string) {
        super(name, "t2.micro");
    }
}

export class Large extends Server {
    constructor(name: string) {
        super(name, "t2.large");
    }
}

