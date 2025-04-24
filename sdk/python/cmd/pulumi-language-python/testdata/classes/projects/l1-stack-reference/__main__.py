import pulumi

ref = pulumi.StackReference("ref", stack_name="organization/other/dev")
pulumi.export("plain", ref.get_output("plain"))
pulumi.export("secret", ref.get_output("secret"))
