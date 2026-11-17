resource "failing" "fail_on_create:index:Resource" {
    value = false
}

resource "dependent" "simple:index:Resource" {
    value = failing.value
}

resource "independent" "simple:index:Resource" {
    value = true
}

output "failingValue" {
    value = failing.value
}
