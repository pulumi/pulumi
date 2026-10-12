import pulumi
import pulumi_fail_on_create as fail_on_create
import pulumi_simple as simple

independent = simple.Resource("independent", value=True)
pulumi.export("recovered", fail_on_create.fail_output(value=independent.value).message.recover(lambda __error: (lambda error: f"recovered: {error}")(str(__error))))
recovered_value = simple.Resource("recovered_value", value=fail_on_create.fail_output(value=independent.value).value.recover(lambda __error: (lambda error: error != "")(str(__error))))
