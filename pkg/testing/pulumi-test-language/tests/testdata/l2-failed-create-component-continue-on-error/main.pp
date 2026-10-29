resource "comp" "failingcomponent:index:Component" {}

resource "dependent" "simple:index:Resource" {
    value = comp.value == "hello"
}

resource "independent" "simple:index:Resource" {
    value = true
}

output "componentValue" {
    value = comp.value
}
