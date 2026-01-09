import pulumi
import pulumi_eks as eks

rawkode = eks.Cluster("rawkode",
    instance_type="t2.medium",
    desired_capacity=2,
    min_size=1,
    max_size=2)
stack72 = eks.Cluster("stack72",
    instance_type="t2.medium",
    desired_capacity=4,
    min_size=1,
    max_size=8)
