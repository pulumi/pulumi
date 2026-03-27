package "subpackage" {
    baseProviderName = "parameterized"
    baseProviderVersion = "1.2.3"
    parameterization {
        name = "subpackage"
        version = "2.0.0"
        value = "SGVsbG9Xb3JsZA==" // base64(utf8_bytes("HelloWorld"))
    }
}

// The resource name is based on the parameter value
resource example "subpackage:index:HelloWorld" { }

resource exampleComponent "subpackage:index:HelloWorldComponent" { }

output "parameterValue" {
    value = example.parameterValue
}

output "parameterValueFromComponent" {
    value = exampleComponent.parameterValue
}