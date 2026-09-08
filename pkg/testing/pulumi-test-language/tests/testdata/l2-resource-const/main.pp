resource "first" "constant:index:Resource" {
    kind = "Constant"
    flag = true
    count = 3
    ratio = 1.5
}

// Every property has a constant value in the schema, one per constant kind; reading them must
// bind without type errors.
output "kind" {
    value = first.kind
}

output "flag" {
    value = first.flag
}

output "count" {
    value = first.count
}

output "ratio" {
    value = first.ratio
}
