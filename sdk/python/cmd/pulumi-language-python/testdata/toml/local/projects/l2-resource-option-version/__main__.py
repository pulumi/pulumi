import pulumi
import pulumi_simple as simple

with_v1 = simple.Resource("withV1", value=True,
opts = pulumi.ResourceOptions(version="2.0.0"))
with_v2 = simple.Resource("withV2", value=False)
with_default = simple.Resource("withDefault", value=True)
