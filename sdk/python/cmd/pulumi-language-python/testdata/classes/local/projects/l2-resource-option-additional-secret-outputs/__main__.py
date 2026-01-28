import pulumi
import pulumi_simple as simple

with_secret = simple.Resource("withSecret", value=True,
opts = pulumi.ResourceOptions(additional_secret_outputs=["value"]))
without_secret = simple.Resource("withoutSecret", value=True)
