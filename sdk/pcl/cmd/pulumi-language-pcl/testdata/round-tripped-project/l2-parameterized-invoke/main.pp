package "subpackage" {
    baseProviderName = "parameterized"
    baseProviderVersion = "1.2.3"
    parameterization {
        name = "subpackage"
        version = "2.0.0"
        value = "SGVsbG9Xb3JsZA==" // base64(utf8_bytes("HelloWorld"))
    }
}

// The invoke name is based on the parameter value
output "parameterValue" {
    value = invoke("subpackage:index:doHelloWorld", {input: "goodbye"}).output
}