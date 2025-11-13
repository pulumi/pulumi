import pulumi
import pulumi_simple as simple

hide_diffs = simple.Resource("hideDiffs", value=True,
opts = pulumi.ResourceOptions(hide_diffs=["value"]))
not_hide_diffs = simple.Resource("notHideDiffs", value=True)
