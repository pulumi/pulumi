resource "res" "conformance-component:index:Simple" {
    value = true
    options {
        replacementTrigger = "trigger-value-updated"
    }
}

resource "simpleResource" "simple:index:Resource" {
    value = false
}
