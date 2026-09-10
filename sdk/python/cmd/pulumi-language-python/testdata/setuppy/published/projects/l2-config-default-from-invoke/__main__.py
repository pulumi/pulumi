import pulumi
import pulumi_simple_invoke as simple_invoke

my_invoke_result = simple_invoke.my_invoke_output(value="hello")
config = pulumi.Config()
default_from_invoke: pulumi.Input[str] | None = config.get("defaultFromInvoke")
if default_from_invoke is None:
    default_from_invoke = my_invoke_result.result
pulumi.export("result", default_from_invoke)
