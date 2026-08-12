resource "target" "simple:index:Resource" {
    value = true
}

resource "other" "nestedobject:index:Container" {
    inputs = ["a"]
}

resource "skipped" "nestedobject:index:Container" {
    inputs = ["b"]
}

output "skippedOutput" {
    value = "skipped-${skipped.details[0].key}"
}
