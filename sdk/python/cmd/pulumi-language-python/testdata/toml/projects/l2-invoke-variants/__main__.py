import pulumi
import pulumi_simple_invoke as simple_invoke

res = simple_invoke.StringResource("res", text="hello")
pulumi.export("outputInput", simple_invoke.my_invoke_output(value=res.text).apply(lambda invoke: invoke.result))
pulumi.export("unit", simple_invoke.unit_output().apply(lambda invoke: invoke.result))
