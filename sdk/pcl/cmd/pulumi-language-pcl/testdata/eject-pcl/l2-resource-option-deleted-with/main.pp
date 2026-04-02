resource "target" "simple:index:Resource" {
    value = true
}

resource "deletedWith" "simple:index:Resource" {
    value = true
    options {
        deletedWith = target
    }
}

resource "notDeletedWith" "simple:index:Resource" {
    value = true
}
