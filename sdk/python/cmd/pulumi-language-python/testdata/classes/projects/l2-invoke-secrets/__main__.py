import pulumi
import pulumi_simple as simple
import pulumi_simple_invoke as simple_invoke

res = simple.Resource("res", value=True)
pulumi.export("nonSecret", simple_invoke.secret_invoke_output(value="hello",
    secret_response=False).apply(lambda invoke: invoke.response))
pulumi.export("firstSecret", simple_invoke.secret_invoke_output(value="hello",
    secret_response=res.value).apply(lambda invoke: invoke.response))
pulumi.export("secondSecret", simple_invoke.secret_invoke_output(value=pulumi.Output.secret("goodbye"),
    secret_response=False).apply(lambda invoke: invoke.response))
