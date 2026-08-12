config input bool {
    description = "A simple input"
}

resource "res" "simple:index:Resource" {
    value = input
}

output output { 
    value = res.value
}