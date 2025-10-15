import pulumi
import pulumi_simple_invoke as simple_invoke

pulumi.export("hello", simple_invoke.my_invoke_output(value="hello").apply(lambda invoke: invoke.result))
pulumi.export("goodbye", simple_invoke.my_invoke_output(value="goodbye").apply(lambda invoke: invoke.result))
