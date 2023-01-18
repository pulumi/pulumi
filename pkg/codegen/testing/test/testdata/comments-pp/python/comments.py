import pulumi
import pulumi_aws as aws

# Test comments for a resource
security_group = aws.ec2.SecurityGroup("securityGroup", ingress=[aws.ec2.SecurityGroupIngressArgs(
    protocol="tcp",
    from_port=0,
    to_port=0,
    cidr_blocks=["0.0.0.0/0"],
)])
ami = aws.get_ami(filters=[aws.GetAmiFilterArgs(
        name="name",
        values=["amzn-ami-hvm-*-x86_64-ebs"],
    )],
    owners=["137112412989"],
    most_recent=True)
# Test comments for another resource
server = aws.ec2.Instance("server",
    tags={
        "Name": "web-server-www",
    },
    instance_type="t2.micro",
    security_groups=[security_group.name],
    ami=ami.id,
    user_data="""#!/bin/bash
echo "Hello, World!" > index.html
nohup python -m SimpleHTTPServer 80 &
""")
# Final trailing resource comment
pulumi.export("secGroupName", security_group.name)
