pulumi {
    requiredVersionRange = ">=3.0.1"
}

component someComponent "./myComponent" {
    input = true
}

output result {
    value = someComponent.output
}
