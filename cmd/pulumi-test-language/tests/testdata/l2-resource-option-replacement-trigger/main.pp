resource "replacementTrigger" "simple:index:Resource" {
    value = true
    options {
        replacementTrigger = "test"
    }
}

resource "notReplacementTrigger" "simple:index:Resource" {
    value = true
}
