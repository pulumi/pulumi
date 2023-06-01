import pulumi
import pulumi_aws as aws
import pulumi_eks as eks

vpc_id = aws.ec2.get_vpc_output(default=True).apply(lambda invoke: invoke.id)
subnet_ids = aws.ec2.get_subnet_ids_output(vpc_id=vpc_id).apply(lambda invoke: invoke.ids)
cluster = eks.Cluster("cluster",
    vpc_id=vpc_id,
    subnet_ids=subnet_ids,
    instance_type="t2.medium",
    desired_capacity=2,
    min_size=1,
    max_size=2)
pulumi.export("kubeconfig", cluster.kubeconfig)
