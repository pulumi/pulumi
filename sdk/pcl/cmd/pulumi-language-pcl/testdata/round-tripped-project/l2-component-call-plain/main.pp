resource "configurer" "configurer:index:Configurer" {
    providerConfig = "propagated"
}

resource "customFromPlainProvider" "configurer:index:Custom" {
    value = "from-plain-provider"

    options {
        provider = call(configurer, "plainProvider", {})
    }
}

resource "customFromNestedPlainProvider" "configurer:index:Custom" {
    value = "from-nested-plain-provider"

    options {
        provider = call(configurer, "nestedPlainProvider", {}).provider
    }
}

output "plainValue" {
    value = call(configurer, "plainValue", {})
}

output "nestedPlainValue" {
    value = call(configurer, "nestedPlainProvider", {}).value
}
