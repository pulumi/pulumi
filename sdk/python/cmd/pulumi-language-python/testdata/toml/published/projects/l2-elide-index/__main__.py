import pulumi
import pulumi_simple as simple
import pulumi_simple_invoke as simple_invoke

res = simple.Resource("res", value=True)
pulumi.export("inv", simple_invoke.my_invoke_output(value="test").apply(lambda invoke: invoke.result))
