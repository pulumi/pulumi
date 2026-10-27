// Tests that an extension resource inherits its parent's provider for the base package.
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

// This should inherit prov from parent, because prov is the base provider of myext.
resource child "myext:index:Greeting" {
    options {
        parent = parent
    }
}
