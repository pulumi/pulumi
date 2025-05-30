resource "component1" "callreturnsprovider:index:ComponentCallable" {}

output "from_identity" {
    value = call(component1, "identity", { value = "bar" })
}
