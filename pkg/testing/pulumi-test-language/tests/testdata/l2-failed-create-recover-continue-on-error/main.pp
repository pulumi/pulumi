resource "failing" "fail_on_create:index:Resource" {
    value = false
}

output "recovered" {
    value = recover(failing.urn, "recovered: ${error}")
}

resource "recovered_value" "simple:index:Resource" {
    value = recover(failing.value, error != "")
}

resource "independent" "simple:index:Resource" {
    value = true
}
