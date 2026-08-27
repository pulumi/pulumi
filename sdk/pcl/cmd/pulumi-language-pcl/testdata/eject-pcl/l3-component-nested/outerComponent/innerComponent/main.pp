config input bool {
    description = "An input passed to the inner component"
}

resource "res" "simple:index:Resource" {
    value = !input
}

output output bool {
    value = res.value
}
