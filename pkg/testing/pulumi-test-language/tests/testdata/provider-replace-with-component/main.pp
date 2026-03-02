resource "target" "conformance-component:index:Simple" {
    value = true
}

resource "replaceWith" "conformance-component:index:Simple" {
    value = true
    options {
        replaceWith = [target]
    }
}

resource "notReplaceWith" "conformance-component:index:Simple" {
    value = true
}

// Ensure the simple plugin is discoverable for this conformance run.
resource "simpleResource" "simple:index:Resource" {
    value = false
}
