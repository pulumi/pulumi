component simpleComponent "./simpleComponent" {}

component multipleSimpleComponents "./simpleComponent" {
    options {
        range = 10
    }
}

component anotherComponent "./another-component" {}

component exampleComponent "./exampleComponent" {
    input = "doggo"
    ipAddress = [127, 0, 0, 1]
    cidrBlocks = {
        "one" = "uno"
        "two" = "dos"
    }
    githubApp = {
        id = "example id"
        keyBase64 = "base64 encoded key"
        webhookSecret = "very important secret"
    }
    servers = [
        { name = "First" },
        { name = "Second" }
    ]
    deploymentZones = {
        "first" = {
            zone = "First zone"
        }, 
        "second" = {
            zone = "Second zone"
        }
    }
}

output result {
    value = exampleComponent.result
}