resource "protected" "simple:index:Resource" {
    value = true
    options {
        protect = true
    }
}

resource "unprotected" "simple:index:Resource" {
    value = true
    options {
        protect = false
    }
}

resource "defaulted" "simple:index:Resource" {
    value = true
}
