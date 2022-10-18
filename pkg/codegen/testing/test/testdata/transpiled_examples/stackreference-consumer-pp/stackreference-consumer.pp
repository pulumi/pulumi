resource stackRef "pulumi:pulumi:StackReference" {
    name = "PLACEHOLDER_ORG_NAME/stackreference-producer/PLACEHOLDER_STACK_NAME"
}

output referencedImageName {
    value = "${stackRef.outputs["imageName"]}"
}
