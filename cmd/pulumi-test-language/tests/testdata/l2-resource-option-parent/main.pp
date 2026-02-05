resource "parent" "simple:index:Resource" {
    value = true
}

resource "withParent" "simple:index:Resource" {
    value = false
    options {
        parent = parent
    }
}

resource "noParent" "simple:index:Resource" {
    value = true
}
