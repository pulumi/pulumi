resource "ignoreChanges" "simple:index:Resource" {
    value = true
    options {
        ignoreChanges = [value]
    }
}

resource "notIgnoreChanges" "simple:index:Resource" {
    value = true
}
