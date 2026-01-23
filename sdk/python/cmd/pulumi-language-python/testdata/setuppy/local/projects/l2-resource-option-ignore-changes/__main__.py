import pulumi
import pulumi_simple as simple

ignore_changes = simple.Resource("ignoreChanges", value=True,
opts = pulumi.ResourceOptions(ignore_changes=["value"]))
not_ignore_changes = simple.Resource("notIgnoreChanges", value=True)
