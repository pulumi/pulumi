// Tests that an extension invoke inherits its parent's provider for the base package.
package {
    baseProviderName = "extbase"
    baseProviderVersion = "45.0.0"
    parameterization {
        name = "myext"
        version = "2.0.0"
        value = "SGVsbG8=" // base64(utf8_bytes("Hello"))
    }
}

resource prov "pulumi:providers:extbase" { }

resource parent "extbase:index:Base" {
    options {
        provider = prov
    }
}

// This should use prov from parent, because prov is the base provider of myext.
greeting = invoke("myext:index:greet", { name = "World" }, {
    parent = parent
})

output "greeting" {
    value = greeting.greeting
}
