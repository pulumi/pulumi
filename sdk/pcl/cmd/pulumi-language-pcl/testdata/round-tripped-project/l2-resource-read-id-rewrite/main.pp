read "res" "read-id-rewrite:index:Resource" {
    id = "existing-id"
    lookup = "existing-key"
}

resource "src" "simple:index:Resource" {
    value = true
}

read "unknownRes" "read-id-rewrite:index:Resource" {
    id = src.id
    lookup = "another-key"
}

output "resourceId" {
    value = res.id
}

output "lookup" {
    value = res.lookup
}

output "unknownResourceId" {
    value = unknownRes.id
}

output "unknownLookup" {
    value = unknownRes.lookup
}
