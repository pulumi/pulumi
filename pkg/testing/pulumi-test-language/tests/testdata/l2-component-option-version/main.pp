resource "withV2" "conformance-component:index:Simple" {
    value = true
    options {
        version = "2.0.0"
    }
}

resource "withV22" "conformance-component:index:Simple" {
    value = false
    options {
        version = "22.0.0"
    }
}

resource "withDefault" "conformance-component:index:Simple" {
    value = true
}

// Keep a simple provider in the program so provider detection is explicit.
resource "simpleResource" "simple:index:Resource" {
    value = false
}
