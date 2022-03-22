import pulumi
import pulumi_aws as aws

web_server = aws.ec2.Instance("webServer", ami=aws.get_ami().id)
