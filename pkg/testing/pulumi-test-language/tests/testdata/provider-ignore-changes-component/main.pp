resource "withIgnoreChanges" "conformance-component:index:Simple" {
    value = true
    options {
        ignoreChanges = [value]
    }
}

resource "withoutIgnoreChanges" "conformance-component:index:Simple" {
    value = true
}

// Make a simple resource so that plugin detection works.
resource "simpleResource" "simple:index:Resource" {
    value = false
}
