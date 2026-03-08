resource "defaultRes" "call:index:Custom" {
    value = "defaultValue"
}

output "defaultProviderValue" {
    value = call(defaultRes, "providerValue", {}).result
}
