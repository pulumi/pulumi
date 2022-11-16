import pulumi
import pulumi_aws as aws

test = aws.fsx.OpenZfsFileSystem("test",
    storage_capacity=64,
    subnet_ids=[aws_subnet["test1"]["id"]],
    deployment_type="SINGLE_AZ_1",
    throughput_capacity=64)
