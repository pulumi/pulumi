// An extension resource (Greeting) and a base-provider resource (Base) used
// together; both live in the base provider's namespace ("extbase").
package {
    baseProviderName = "extbase"
    baseProviderVersion = "45.0.0"
    parameterization {
        name = "myext"
        version = "2.0.0"
        value = "SGVsbG8=" // base64(utf8_bytes("Hello"))
    }
}

resource greeting "extbase:index:Greeting" { }

resource base "extbase:index:Base" { }

output "parameterValue" {
    value = greeting.parameterValue
}

output "baseValue" {
    value = base.baseValue
}
