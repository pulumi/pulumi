resource "a" "simple:index:Resource" {
    value = true
}

resource "b" "simple:index:Resource" {
    value = a.value
}

resource "c" "simple:index:Resource" {
    value = false
    options {
        dependsOn = [a]
    }
}
