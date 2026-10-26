read "failing" "read:index:Resource" {
    id = "existing-id"
    lookup = "fail"
}

output "failingValue" {
    value = failing.value
}

output "failingLookup" {
    value = failing.lookup
}
