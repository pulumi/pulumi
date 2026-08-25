resource "src" "read:index:Resource" {
    value = true
}

read "res" "read:index:Resource" {
    id = src.id
    lookup = "existing-key"
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
