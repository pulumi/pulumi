import pulumi
import pulumi_aws as aws

web_security_group = aws.ec2.SecurityGroup("webSecurityGroup", vpc_id=aws.ec2.get_vpc(default=True).id)
