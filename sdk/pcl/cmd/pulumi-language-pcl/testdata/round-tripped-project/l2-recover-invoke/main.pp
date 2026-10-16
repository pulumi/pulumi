resource "independent" "simple:index:Resource" {
    value = true
}

output "recovered" {
    value = recover(invoke("fail_on_create:index:fail", { value = independent.value }).message, "recovered: ${error}")
}

resource "recovered_value" "simple:index:Resource" {
    value = recover(invoke("fail_on_create:index:fail", { value = independent.value }).value, error != "")
}
