import pulumi
import pulumi_simple_invoke as simple_invoke

provider = simple_invoke.Provider("provider")
parent = simple_invoke.StringResource("parent", text="parent",
opts = pulumi.ResourceOptions(provider=provider))
# This should inherit the explicit provider from parent.
child = simple_invoke.StringResource("child", text="child",
opts = pulumi.ResourceOptions(parent=parent))
# This should use the explicit provider of parent.
parent_data = simple_invoke.my_invoke_output(value="parent", opts=pulumi.InvokeOutputOptions(parent=parent))
# This should use the explicit provider that child inherited from parent.
child_data = simple_invoke.my_invoke_output(value="child", opts=pulumi.InvokeOutputOptions(parent=child))
pulumi.export("parentResult", parent_data.result)
pulumi.export("childResult", child_data.result)
