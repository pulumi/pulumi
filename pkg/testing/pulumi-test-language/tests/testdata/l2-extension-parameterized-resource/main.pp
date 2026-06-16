// Extension parameterization: the SDK is published as "myext" but the resource
// tokens live in the base provider's namespace ("extbase").
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

resource greetingComp "extbase:index:GreetingComponent" { }

output "parameterValue" {
    value = greeting.parameterValue
}

output "parameterValueFromComponent" {
    value = greetingComp.parameterValue
}

output "invokeGreeting" {
    value = invoke("extbase:index:greet", { name = "Pulumi" }).greeting
}
