component simpleComponent "./simpleComponent" {}

component exampleComponent "./exampleComponent" {
    input = "doggo"
}

output result {
    value = exampleComponent.result
}