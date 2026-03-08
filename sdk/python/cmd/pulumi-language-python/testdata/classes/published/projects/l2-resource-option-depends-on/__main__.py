import pulumi
import pulumi_simple as simple

no_depends_on = simple.Resource("noDependsOn", value=True)
with_depends_on = simple.Resource("withDependsOn", value=False,
opts = pulumi.ResourceOptions(depends_on=[no_depends_on]))
