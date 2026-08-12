resource "res" "conformance-component:index:Simple" {
    value = true
}

// Make a simple resource so that plugin detection works.
resource "simpleResource" "simple:index:Resource" {
    value = false
}