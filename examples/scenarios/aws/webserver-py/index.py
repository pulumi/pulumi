# Copyright 2017 Pulumi, Inc. All rights reserved.

import lumi.aws

size = "t2.micro"

def main():
    group = aws.ec2.SecurityGroup("web-secgrp",
            group_description="Enable HTTP access",
            security_group_ingress=[
                aws.ec2.SecurityGroupIngressRule("tcp", 80, 80, "0.0.0.0/0")
            ])
    server = aws.ec2.Instance("web-server-www",
            instance_type=size,
            security_groups=[ group ],
            image_id=aws.ec2.get_linux_ami(size))

if __name__ == "__main__":
    main()

