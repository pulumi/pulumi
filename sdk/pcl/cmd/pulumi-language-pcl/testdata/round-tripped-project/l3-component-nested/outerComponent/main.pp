config input bool {
    description = "An input passed to the outer component"
}

component innerComponent "./innerComponent" {
    input = !input
}

output output bool {
    value = innerComponent.output
}
