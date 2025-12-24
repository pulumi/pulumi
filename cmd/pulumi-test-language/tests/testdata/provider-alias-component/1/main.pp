// Make a simple resource to use as a parent
resource "parent" "simple:index:Resource" {
    value = true
}

// parent "res" to a new parent and alias it so it doesn't recreate.
resource "res" "conformance-component:index:Simple" {
    value = true
    options {
        parent = resource.res
        aliases = [{
            noParent = true
        }]
    }
}

// Make a simple resource so that plugin detection works.
resource "simpleResource" "simple:index:Resource" {
    value = false
}