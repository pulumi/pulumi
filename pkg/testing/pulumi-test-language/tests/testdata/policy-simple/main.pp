resource "res1" "simple:index:Resource" {
    value = true
}

resource "res2" "simple:index:Resource" {
    value = !res1.value
}