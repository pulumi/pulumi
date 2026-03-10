resource "retainOnDelete" "simple:index:Resource" {
    value = true
    options {
        retainOnDelete = true
    }
}

resource "notRetainOnDelete" "simple:index:Resource" {
    value = true
    options {
        retainOnDelete = false
    }
}

resource "defaulted" "simple:index:Resource" {
    value = true
}
