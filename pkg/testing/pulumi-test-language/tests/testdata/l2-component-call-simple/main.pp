resource "component1" "component:index:ComponentCallable" {
    value = "bar"
}

output "from_identity" {
    value = call(component1, "identity", {}).result
}

output "from_prefixed" {
    value = call(component1, "prefixed", { prefix = "foo-" }).result
}
