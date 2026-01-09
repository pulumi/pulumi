import pulumi
import pulumi_aws as aws

config = pulumi.Config()
instance_type = config.get("instanceType")
if instance_type is None:
    instance_type = "t2.micro"
ami = aws.ec2.get_ami(filters=[{
        "name": "name",
        "values": ["amzn-ami-hvm-*"],
    }],
    owners=["137112412989"],
    most_recent=True).id
user_data = """#!/bin/bash
echo "Hello, World from Pulumi!" > index.html
nohup python -m SimpleHTTPServer 80 &"""
sec_group = aws.ec2.SecurityGroup("secGroup",
    description="Enable HTTP access",
    ingress=[{
        "from_port": 80,
        "to_port": 80,
        "protocol": "tcp",
        "cidr_blocks": ["0.0.0.0/0"],
    }],
    tags={
        "Name": "web-secgrp",
    })
server = aws.ec2.Instance("server",
    instance_type=instance_type,
    vpc_security_group_ids=[sec_group.id],
    user_data=user_data,
    ami=ami,
    tags={
        "Name": "web-server-www",
    })
pulumi.export("publicIP", server.public_ip)
pulumi.export("publicDNS", server.public_dns)
