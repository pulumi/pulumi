component outerComponent "./outerComponent" {
    input = true
}

output result {
    value = outerComponent.output
}
