// Extension parameterization: the SDK is published as "myext" and the resource
// tokens live in the extension's own namespace.
package {
    baseProviderName = "extbase"
    baseProviderVersion = "45.0.0"
    parameterization {
        name = "myext"
        version = "2.0.0"
        value = "SGVsbG8=" // base64(utf8_bytes("Hello"))
    }
}

resource greeting "myext:index:Greeting" { }

resource greetingComp "myext:index:GreetingComponent" { }

output "parameterValue" {
    value = greeting.parameterValue
}

output "parameterValueFromComponent" {
    value = greetingComp.parameterValue
}

output "invokeGreeting" {
    value = invoke("myext:index:greet", { name = "Pulumi" }).greeting
}
