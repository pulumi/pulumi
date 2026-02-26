config input bool {
    description = "A simple input"
}

resource "localChild" "simple:index:Resource" {
    value = input
}

output output {
    value = localChild.value
}
