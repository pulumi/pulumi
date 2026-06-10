resource "first" "constant:index:Resource" {
    kind = "Constant"
}

// `kind` has a constant value in the schema; reading it must bind without type errors.
output "kind" {
    value = first.kind
}
