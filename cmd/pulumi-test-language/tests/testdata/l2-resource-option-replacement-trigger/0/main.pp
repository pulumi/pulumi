resource "replacementTrigger" "simple:index:Resource" {
    value = true
    options {
        replacementTrigger = "test"
    }
}

resource "unknown" "output:index:Resource" {
    value = 1
}

resource "unknownReplacementTrigger" "simple:index:Resource" {
    value = true
    options {
        replacementTrigger = "hellohello"
    }
}

resource "notReplacementTrigger" "simple:index:Resource" {
    value = true
}

resource "secretReplacementTrigger" "simple:index:Resource" {
    value = true
    options {
        replacementTrigger = secret([1, 2, 3])
    }
}