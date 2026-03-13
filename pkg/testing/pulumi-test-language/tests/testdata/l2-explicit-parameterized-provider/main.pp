package "goodbye" {
    baseProviderName = "parameterized"
    baseProviderVersion = "1.2.3"
    parameterization {
        name = "goodbye"
        version = "2.0.0"
        value = "R29vZGJ5ZQ==" // base64(utf8_bytes("Goodbye"))
    }
}

resource "prov" "pulumi:providers:goodbye" {
    text = "World"
}

// The resource name is based on the parameter value
resource "res" "goodbye:index:Goodbye" {
    options {
        provider = prov
    }
}

// The resource name is based on the parameter value and the provider config
output "parameterValue" {
    value = res.parameterValue
}