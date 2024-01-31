import pulumi
import pulumi_simple as simple

target = simple.Resource("target", value=True)
other = simple.Resource("other", value=True)
