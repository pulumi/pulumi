resource "explicitProv" "pulumi:providers:call" {
    value = "explicitProvValue"
}

resource "explicitRes" "call:index:Custom" {
    value = "explicitValue"

    options {
        provider = explicitProv
    }
}

output "explicitProviderValue" {
    value = call(explicitRes, "providerValue", {}).result
}

output "explicitProvFromIdentity" {
    value = call(explicitProv, "identity", {}).result
}

output "explicitProvFromPrefixed" {
    value = call(explicitProv, "prefixed", { prefix = "call-prefix-" }).result
}
