import pulumi
import pulumi_aws as aws
import pulumi_eks as eks

vpc_id = aws.ec2.get_vpc(default=True).id
subnet_ids = aws.ec2.get_subnet_ids(vpc_id=vpc_id).ids
cluster = eks.Cluster("cluster",
    vpc_id=vpc_id,
    subnet_ids=subnet_ids,
    instance_type="t2.medium",
    desired_capacity=2,
    min_size=1,
    max_size=2)
pulumi.export("kubeconfig", cluster.kubeconfig)
