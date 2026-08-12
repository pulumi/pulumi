import pulumi
import pulumi_simple as simple

target_only = simple.Resource("targetOnly", value=True)
unrelated = simple.Resource("unrelated", value=True)
