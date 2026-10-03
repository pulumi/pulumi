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

read "res_prop_dep" "read:index:Resource" {
    id = "existing-id"
    lookup = "existing-key-${failing.value}"
}

output "readValue" {
    value = res.value
}

output "readPropDepValue" {
    value = res_prop_dep.value
}
