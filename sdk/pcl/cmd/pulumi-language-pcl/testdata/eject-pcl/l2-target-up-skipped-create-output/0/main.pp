resource "target" "simple:index:Resource" {
    value = true
}

resource "other" "nestedobject:index:Container" {
    inputs = ["a"]
}
