resource "targetOnly" "simple:index:Resource" {
    value = true
}

resource "unrelated" "simple:index:Resource" {
    value = true
    options {
        dependsOn = [dep]
    }
}

resource "dep" "simple:index:Resource" {
    value = true
}
