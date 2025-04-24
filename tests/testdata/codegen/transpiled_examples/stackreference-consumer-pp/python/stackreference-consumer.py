import pulumi

stack_ref = pulumi.StackReference("stackRef", stack_name="PLACEHOLDER_ORG_NAME/stackreference-producer/PLACEHOLDER_STACK_NAME")
pulumi.export("referencedImageName", stack_ref.outputs["imageName"])
