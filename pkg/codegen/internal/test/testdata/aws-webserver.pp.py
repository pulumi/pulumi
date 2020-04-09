import pulumi
import json
import pulumi_aws as aws

# Create a new security group for port 80.
security_group = aws.ec2.SecurityGroup("security_group", ingress=[{
    "protocol": "tcp",
    "fromPort": 0,
    "toPort": 0,
    "cidrBlocks": ["0.0.0.0/0"],
}])
ami = aws.get_ami({
    "filters": [{
        "name": "name",
        "values": ["amzn-ami-hvm-*-x86_64-ebs"],
    }],
    "owners": ["137112412989"],
    "mostRecent": True,
})
# Create a simple web server using the startup script for the instance.
server = aws.ec2.Instance("server",
    tags={
        "Name": "web-server-www",
    },
    instance_type="t2.micro",
    security_groups=[security_group.name],
    ami=ami.id,
    user_data=f"""#!/bin/bash
echo \"Hello, World!\" > index.html
nohup python -m SimpleHTTPServer 80 &
""")
pulumi.export("publicIp", server.publicIp)
pulumi.export("publicHostName", server.publicDns)
