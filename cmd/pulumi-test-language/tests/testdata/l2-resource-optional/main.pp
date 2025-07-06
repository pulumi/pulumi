
resource "resA" "simple:index:Resource" {
    value = true
}

resource "resB" "simple-optional:index:Resource" {
    // Can set an optional input from a required output
    value = resA.value
}

resource "resC" "simple-optional:index:Resource" {
    // Can set an optional input from an optional output
    value = resB.value
    // Can set an optional input directly to null
    text = null
}
