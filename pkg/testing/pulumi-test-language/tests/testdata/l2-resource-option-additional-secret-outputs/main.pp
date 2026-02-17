resource "withSecret" "simple:index:Resource" {
    value = true
    options {
        additionalSecretOutputs = ["value"]
    }
}

resource "withoutSecret" "simple:index:Resource" {
    value = true
}
