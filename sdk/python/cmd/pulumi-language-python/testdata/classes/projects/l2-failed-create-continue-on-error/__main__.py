import pulumi
import pulumi_fail_on_create as fail_on_create
import pulumi_simple as simple

failing = fail_on_create.Resource("failing", value=False)
dependent = simple.Resource("dependent", value=True,
opts = pulumi.ResourceOptions(depends_on=[failing]))
dependent_on_output = simple.Resource("dependent_on_output", value=failing.value)
independent = simple.Resource("independent", value=True)
double_dependency = simple.Resource("double_dependency", value=True,
opts = pulumi.ResourceOptions(depends_on=[
        independent,
        dependent_on_output,
    ]))
