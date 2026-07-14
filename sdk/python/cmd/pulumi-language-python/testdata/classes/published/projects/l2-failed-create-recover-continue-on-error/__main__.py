import pulumi
import pulumi_fail_on_create as fail_on_create
import pulumi_simple as simple

failing = fail_on_create.Resource("failing", value=False)
pulumi.export("recovered", failing.urn.recover(lambda __error: (lambda error: f"recovered: {error}")(str(__error))))
recovered_value = simple.Resource("recovered_value", value=failing.value.recover(lambda __error: (lambda error: error != "")(str(__error))))
independent = simple.Resource("independent", value=True)
