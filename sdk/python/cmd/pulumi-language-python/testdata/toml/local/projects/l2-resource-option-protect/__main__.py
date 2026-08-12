import pulumi
import pulumi_simple as simple

protected = simple.Resource("protected", value=True,
opts = pulumi.ResourceOptions(protect=True))
unprotected = simple.Resource("unprotected", value=True,
opts = pulumi.ResourceOptions(protect=False))
defaulted = simple.Resource("defaulted", value=True)
