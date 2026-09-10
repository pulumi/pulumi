config "extraCount" "int" {}

resource "res1" "simple:index:Resource" {
    value = true
}

resource "res2" "simple:index:Resource" {
    options { range = extraCount }
    value = false
}
