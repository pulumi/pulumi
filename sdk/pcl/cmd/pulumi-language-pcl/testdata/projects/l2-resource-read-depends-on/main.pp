resource "src" "simple:index:Resource" {
    value = true
}

read "res" "read:index:Resource" {
    id = "existing-id"
    lookup = "existing-key"
    options {
        dependsOn = [src]
    }
}

output "resourceUrn" {
    value = res.urn
}

output "resourceId" {
    value = res.id
}

output "lookup" {
    value = res.lookup
}

output "value" {
    value = res.value
}
