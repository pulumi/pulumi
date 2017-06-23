// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as aws from "@lumi/aws";

let group = new aws.ec2.SecurityGroup("web-secgrp", {
    groupDescription: "Enable HTTP access",
    securityGroupIngress: [
        { ipProtocol: "tcp", fromPort: 80, toPort: 80, cidrIp: "0.0.0.0/0" },
    ],
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

export class Nano extends Server {
    constructor(name: string) {
        super(name, "t2.nano");
    }
}

