import pulumi
import pulumi_output_only_invoke as output_only_invoke

pulumi.export("hello", output_only_invoke.my_invoke_output(value="hello").apply(lambda invoke: invoke.result))
pulumi.export("goodbye", output_only_invoke.my_invoke_output(value="goodbye").apply(lambda invoke: invoke.result))
