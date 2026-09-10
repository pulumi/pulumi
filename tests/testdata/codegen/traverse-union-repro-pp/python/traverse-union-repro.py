import pulumi
import pulumi_infra as infra

test = infra.FileSystem("test",
    storage_capacity=64,
    subnet_ids=[aws_subnet["test1"]["id"]],
    deployment_type="SINGLE_AZ_1",
    throughput_capacity=64)
