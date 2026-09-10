import pulumi
import pulumi_output_only_invoke as output_only_invoke

pulumi.export("hello", output_only_invoke.my_invoke_output(value="hello").result)
pulumi.export("goodbye", output_only_invoke.my_invoke_output(value="goodbye").result)
