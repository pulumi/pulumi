component simpleComponent "./simpleComponent" {}

component exampleComponent "./exampleComponent" {
    input = "doggo"
    ipAddress = [127, 0, 0, 1]
    cidrBlocks = {
        "one" = "uno"
        "two" = "dos"
    }
}

output result {
    value = exampleComponent.result
}