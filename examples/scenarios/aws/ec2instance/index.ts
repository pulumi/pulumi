import * as mu from '@mu/mu';
import { InternetGateway, Instance, SecurityGroup } from '@mu/aws/ec2';

let keyName = "lukehoban-us-east-1";
let instanceType = "t2.micro";
let sshLocation = "0.0.0.0";
let sshLocationCIDR = sshLocation + "/0";
let region = "us-east-1";

let awsRegionArch2AMI: { [name: string]: { [key: string]: string; } }= {
    "us-east-1": {
        "PV64": "ami-2a69aa47",
        "HVM64": "ami-6869aa05",
        "HVMG2": "ami-648d9973"
    },
    "us-west-2": {
        "PV64": "ami-7f77b31f",
        "HVM64": "ami-7172b611",
        "HVMG2": "ami-09cd7a69"
    },
    "us-west-1": {
        "PV64": "ami-a2490dc2",
        "HVM64": "ami-31490d51",
        "HVMG2": "ami-1e5f0e7e"
    },
    "eu-west-1": {
        "PV64": "ami-4cdd453f",
        "HVM64": "ami-f9dd458a",
        "HVMG2": "ami-b4694ac7"
    },
    "eu-west-2": {
        "PV64": "NOT_SUPPORTED",
        "HVM64": "ami-886369ec",
        "HVMG2": "NOT_SUPPORTED"
    },
    "eu-central-1": {
        "PV64": "ami-6527cf0a",
        "HVM64": "ami-ea26ce85",
        "HVMG2": "ami-de5191b1"
    },
    "ap-northeast-1": {
        "PV64": "ami-3e42b65f",
        "HVM64": "ami-374db956",
        "HVMG2": "ami-df9ff4b8"
    },
    "ap-northeast-2": {
        "PV64": "NOT_SUPPORTED",
        "HVM64": "ami-2b408b45",
        "HVMG2": "NOT_SUPPORTED"
    },
    "ap-southeast-1": {
        "PV64": "ami-df9e4cbc",
        "HVM64": "ami-a59b49c6",
        "HVMG2": "ami-8d8d23ee"
    },
    "ap-southeast-2": {
        "PV64": "ami-63351d00",
        "HVM64": "ami-dc361ebf",
        "HVMG2": "ami-cbaf94a8"
    },
    "ap-south-1": {
        "PV64": "NOT_SUPPORTED",
        "HVM64": "ami-ffbdd790",
        "HVMG2": "ami-decdbab1"
    },
    "us-east-2": {
        "PV64": "NOT_SUPPORTED",
        "HVM64": "ami-f6035893",
        "HVMG2": "NOT_SUPPORTED"
    },
    "ca-central-1": {
        "PV64": "NOT_SUPPORTED",
        "HVM64": "ami-730ebd17",
        "HVMG2": "NOT_SUPPORTED"
    },
    "sa-east-1": {
        "PV64": "ami-1ad34676",
        "HVM64": "ami-6dd04501",
        "HVMG2": "NOT_SUPPORTED"
    },
    "cn-north-1": {
        "PV64": "ami-77559f1a",
        "HVM64": "ami-8e6aa0e3",
        "HVMG2": "NOT_SUPPORTED"
    }
};

let awsInstanceType2Arch: { [name: string]: { Arch: string; } } = {
    "t1.micro": {
        "Arch": "PV64"
    },
    "t2.nano": {
        "Arch": "HVM64"
    },
    "t2.micro": {
        "Arch": "HVM64"
    },
    "t2.small": {
        "Arch": "HVM64"
    },
    "t2.medium": {
        "Arch": "HVM64"
    },
    "t2.large": {
        "Arch": "HVM64"
    },
    "m1.small": {
        "Arch": "PV64"
    },
    "m1.medium": {
        "Arch": "PV64"
    },
    "m1.large": {
        "Arch": "PV64"
    },
    "m1.xlarge": {
        "Arch": "PV64"
    },
    "m2.xlarge": {
        "Arch": "PV64"
    },
    "m2.2xlarge": {
        "Arch": "PV64"
    },
    "m2.4xlarge": {
        "Arch": "PV64"
    },
    "m3.medium": {
        "Arch": "HVM64"
    },
    "m3.large": {
        "Arch": "HVM64"
    },
    "m3.xlarge": {
        "Arch": "HVM64"
    },
    "m3.2xlarge": {
        "Arch": "HVM64"
    },
    "m4.large": {
        "Arch": "HVM64"
    },
    "m4.xlarge": {
        "Arch": "HVM64"
    },
    "m4.2xlarge": {
        "Arch": "HVM64"
    },
    "m4.4xlarge": {
        "Arch": "HVM64"
    },
    "m4.10xlarge": {
        "Arch": "HVM64"
    },
    "c1.medium": {
        "Arch": "PV64"
    },
    "c1.xlarge": {
        "Arch": "PV64"
    },
    "c3.large": {
        "Arch": "HVM64"
    },
    "c3.xlarge": {
        "Arch": "HVM64"
    },
    "c3.2xlarge": {
        "Arch": "HVM64"
    },
    "c3.4xlarge": {
        "Arch": "HVM64"
    },
    "c3.8xlarge": {
        "Arch": "HVM64"
    },
    "c4.large": {
        "Arch": "HVM64"
    },
    "c4.xlarge": {
        "Arch": "HVM64"
    },
    "c4.2xlarge": {
        "Arch": "HVM64"
    },
    "c4.4xlarge": {
        "Arch": "HVM64"
    },
    "c4.8xlarge": {
        "Arch": "HVM64"
    },
    "g2.2xlarge": {
        "Arch": "HVMG2"
    },
    "g2.8xlarge": {
        "Arch": "HVMG2"
    },
    "r3.large": {
        "Arch": "HVM64"
    },
    "r3.xlarge": {
        "Arch": "HVM64"
    },
    "r3.2xlarge": {
        "Arch": "HVM64"
    },
    "r3.4xlarge": {
        "Arch": "HVM64"
    },
    "r3.8xlarge": {
        "Arch": "HVM64"
    },
    "i2.xlarge": {
        "Arch": "HVM64"
    },
    "i2.2xlarge": {
        "Arch": "HVM64"
    },
    "i2.4xlarge": {
        "Arch": "HVM64"
    },
    "i2.8xlarge": {
        "Arch": "HVM64"
    },
    "d2.xlarge": {
        "Arch": "HVM64"
    },
    "d2.2xlarge": {
        "Arch": "HVM64"
    },
    "d2.4xlarge": {
        "Arch": "HVM64"
    },
    "d2.8xlarge": {
        "Arch": "HVM64"
    },
    "hi1.4xlarge": {
        "Arch": "HVM64"
    },
    "hs1.8xlarge": {
        "Arch": "HVM64"
    },
    "cr1.8xlarge": {
        "Arch": "HVM64"
    },
    "cc2.8xlarge": {
        "Arch": "HVM64"
    }
};

let securityGroup = new SecurityGroup("group", {
    groupDescription: "Enable SSH access",
    securityGroupIngress: [{
        ipProtocol: "tcp",
        fromPort: 22,
        toPort: 22,
        cidrIp: sshLocationCIDR,
    }]
});

let instance = new Instance("instance", {
    instanceType: instanceType,
    securityGroups: [securityGroup],
    keyName: keyName,
    imageId: awsRegionArch2AMI[region][awsInstanceType2Arch[instanceType].Arch]
});

