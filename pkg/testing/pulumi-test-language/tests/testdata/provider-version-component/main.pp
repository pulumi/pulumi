resource "withV22" "conformance-component:index:Simple" {
    value = true
    options {
        version = "22.0.0"
    }
}

resource "withDefault" "conformance-component:index:Simple" {
    value = true
}

// Ensure the simple plugin is discoverable for this conformance run.
resource "simpleResource" "simple:index:Resource" {
    value = false
}
