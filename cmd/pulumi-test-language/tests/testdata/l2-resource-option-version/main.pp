resource "withV1" "simple:index:Resource" {
    value = true
    options {
        version = "2.0.0"
    }
}

resource "withV2" "simple:index:Resource" {
    value = false
    options {
        version = "26.0.0"
    }
}

resource "withDefault" "simple:index:Resource" {
    value = true
}
