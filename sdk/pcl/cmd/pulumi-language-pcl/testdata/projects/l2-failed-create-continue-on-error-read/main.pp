resource "failing" "fail_on_create:index:Resource" {
    value = false
}

read "res" "read:index:Resource" {
    id = "existing-id"
    lookup = "existing-key"
    options {
        dependsOn = [failing]
    }
}

output "readValue" {
    value = res.value
}
