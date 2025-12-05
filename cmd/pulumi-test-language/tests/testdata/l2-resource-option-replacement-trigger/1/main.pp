resource "replacementTrigger" "simple:index:Resource" {
    value = true
    options {
        replacementTrigger = "test2"
    }
}

resource "unknown" "output:index:Resource" {
    value = 2
}

resource "unknownReplacementTrigger" "simple:index:Resource" {
    value = true
    options {
        replacementTrigger = unknown.output
    }
}

resource "notReplacementTrigger" "simple:index:Resource" {
    value = true
}
