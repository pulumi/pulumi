import pulumi
import pulumi_fail_on_create as fail_on_create
import pulumi_simple as simple

failing = fail_on_create.Resource("failing", value=False)
dependent = simple.Resource("dependent", value=failing.value)
independent = simple.Resource("independent", value=True)
pulumi.export("failingValue", failing.value)
