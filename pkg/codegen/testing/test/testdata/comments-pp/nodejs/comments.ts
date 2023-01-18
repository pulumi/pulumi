import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

// Test comments for a resource
const securityGroup = new aws.ec2.SecurityGroup("securityGroup", {ingress: [{
    protocol: "tcp",
    fromPort: 0,
    toPort: 0,
    cidrBlocks: ["0.0.0.0/0"],
}]});
const ami = aws.getAmi({
    filters: [{
        name: "name",
        values: ["amzn-ami-hvm-*-x86_64-ebs"],
    }],
    owners: ["137112412989"],
    mostRecent: true,
});
// Test comments for another resource
const server = new aws.ec2.Instance("server", {
    tags: {
        Name: "web-server-www",
    },
    instanceType: "t2.micro",
    securityGroups: [securityGroup.name],
    ami: ami.then(ami => ami.id),
    userData: `#!/bin/bash
echo "Hello, World!" > index.html
nohup python -m SimpleHTTPServer 80 &
`,
});
// Final trailing resource comment
export const secGroupName = securityGroup.name;
