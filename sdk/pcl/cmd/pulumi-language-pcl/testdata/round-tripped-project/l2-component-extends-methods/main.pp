resource "derived" "inherit:index:Derived" {
    message = "hi"
    scale = 1
}

output "status" {
    value = call(derived, "getStatus", {}).status
}
