import pulumi
import pulumi_simple as simple

target = simple.Resource("target", value=True)
deleted_with = simple.Resource("deletedWith", value=True,
opts = pulumi.ResourceOptions(deleted_with=target))
not_deleted_with = simple.Resource("notDeletedWith", value=True)
