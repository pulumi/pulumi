import pulumi

pulumi.export("output_true", True)
pulumi.export("output_false", False)
pulumi.export("output_number", 4)
pulumi.export("output_string", "hello")
