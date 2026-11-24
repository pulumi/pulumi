read "res" "read:index:Resource" {
    id = "existing-id"
    lookup = "existing-key"
}

output "resourceId" {
    value = res.id
}

output "resourceUrn" {
    value = res.urn
}

output "lookup" {
    value = res.lookup
}

output "value" {
    value = res.value
}
