resource "target" "simple:index:Resource" {
    value = true
}

resource "replaceWith" "simple:index:Resource" {
    value = true
    options {
        replaceWith = [target]
    }
}

resource "notReplaceWith" "simple:index:Resource" {
    value = true
}
