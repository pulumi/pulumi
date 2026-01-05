import pulumi
import pulumi_simple as simple

retain_on_delete = simple.Resource("retainOnDelete", value=True,
opts = pulumi.ResourceOptions(retain_on_delete=True))
not_retain_on_delete = simple.Resource("notRetainOnDelete", value=True,
opts = pulumi.ResourceOptions(retain_on_delete=False))
defaulted = simple.Resource("defaulted", value=True)
