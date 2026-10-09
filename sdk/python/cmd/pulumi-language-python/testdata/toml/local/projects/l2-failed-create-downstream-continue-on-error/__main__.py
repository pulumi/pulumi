import pulumi
import pulumi_fail_on_create as fail_on_create
import pulumi_simple as simple

failing = fail_on_create.Resource("failing", value=False)
independent = simple.Resource("independent", value=True)
pulumi.export("failingValue", failing.value)
