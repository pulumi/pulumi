resource "hideDiffs" "simple:index:Resource" {
    value = true
    options {
        hideDiffs = [value]
    }
}

resource "notHideDiffs" "simple:index:Resource" {
    value = true
}
