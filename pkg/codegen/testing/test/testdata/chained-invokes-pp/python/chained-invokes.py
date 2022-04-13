import pulumi
import pulumi_aws as aws

vpc_id = aws.ec2.get_vpc().id
subnet_ids = aws.ec2.get_subnet_ids().ids
