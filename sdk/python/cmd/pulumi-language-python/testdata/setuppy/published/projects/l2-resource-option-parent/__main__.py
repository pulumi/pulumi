import pulumi
import pulumi_simple as simple

parent = simple.Resource("parent", value=True)
with_parent = simple.Resource("withParent", value=False,
opts = pulumi.ResourceOptions(parent=parent))
no_parent = simple.Resource("noParent", value=True)
