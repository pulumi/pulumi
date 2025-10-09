import pulumi
import pulumi_simple_invoke_with_scalar_return as simple_invoke_with_scalar_return

pulumi.export("scalar", simple_invoke_with_scalar_return.my_invoke_scalar_output(value="goodbye"))
