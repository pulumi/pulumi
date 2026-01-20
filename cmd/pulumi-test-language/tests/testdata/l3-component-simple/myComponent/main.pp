config input bool {
    description = "A simple input"
}

resource "res" "simple:index:Resource" {
    value = input
}

output output {
    value = res.value
}

pulumi {
    requiredVersionRange = ">=3.0.2"
}
