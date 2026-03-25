import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as tls from "@pulumi/tls";
import * as fs from "fs";

// Look up the latest Ubuntu 24.04 AMI.
const ami = aws.ec2.getAmiOutput({
    mostRecent: true,
    owners: ["099720109477"], // Canonical
    filters: [
        { name: "name", values: ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"] },
        { name: "virtualization-type", values: ["hvm"] },
    ],
});

// Generate an SSH key pair.
const sshKey = new tls.PrivateKey("ssh-key", {
    algorithm: "ED25519",
});

const keyPair = new aws.ec2.KeyPair("test-runner-key", {
    publicKey: sshKey.publicKeyOpenssh,
});

// Security group allowing SSH access.
const securityGroup = new aws.ec2.SecurityGroup("test-runner-sg", {
    description: "Allow SSH access for remote test runner",
    ingress: [
        {
            protocol: "tcp",
            fromPort: 22,
            toPort: 22,
            cidrBlocks: ["0.0.0.0/0"],
        },
    ],
    egress: [
        {
            protocol: "-1",
            fromPort: 0,
            toPort: 0,
            cidrBlocks: ["0.0.0.0/0"],
        },
    ],
});

// Read userdata script.
const userdata = fs.readFileSync("userdata.sh", "utf-8");

// Create the EC2 instance.
const instance = new aws.ec2.Instance("test-runner", {
    ami: ami.id,
    instanceType: "c5.4xlarge",
    keyName: keyPair.keyName,
    vpcSecurityGroupIds: [securityGroup.id],
    userData: userdata,
    rootBlockDevice: {
        volumeSize: 100,
        volumeType: "gp3",
    },
    tags: {
        Name: "pulumi-test-runner",
    },
});

// Stack outputs.
export const instanceIp = instance.publicIp;
export const privateKey = pulumi.secret(sshKey.privateKeyOpenssh);
export const username = "testrunner";
