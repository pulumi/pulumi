resource "component1" "componentreturnscalar:index:ComponentCallable" {
    value = "bar"
}

output "from_identity" {
    value = call(component1, "identity", {})
}

output "from_prefixed" {
    value = call(component1, "prefixed", { prefix = "foo-" })
}
