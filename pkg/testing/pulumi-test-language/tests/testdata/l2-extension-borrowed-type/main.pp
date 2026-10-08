// An extension resource whose property type is defined by the base provider
// rather than by the extension. The extension schema names the base package in
// its dependencies, so an SDK generator has to record that dependency in the
// generated package's manifest; otherwise the borrowed type does not resolve
// and the generated SDK fails to build.
package {
    baseProviderName = "borrowbase"
    baseProviderVersion = "46.0.0"
    parameterization {
        name = "myborrow"
        version = "3.0.0"
        value = "V2lkZ2V0" // base64(utf8_bytes("Widget"))
    }
}

resource widget "myborrow:index:Widget" { }

output "metadataName" {
    value = widget.metadata.name
}
