// No provider options here: the providers map must be inherited from the
// enclosing local component and flow through the remote component's
// registration into its construct call.
resource "mlc" "component:index:ComponentForeignChild" {
    value = true
}

output "result" {
    value = mlc.value
}
