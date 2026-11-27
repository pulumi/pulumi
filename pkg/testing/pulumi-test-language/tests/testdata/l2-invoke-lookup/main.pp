resource "widget" "lookup-invoke:index:Widget" {
    name = "resource"
}

// getWidget is a root-module function whose name clashes with the Widget
// resource's getter in some SDKs (the Go SDK exports it as LookupWidget).
fromLiteral = invoke("lookup-invoke:index:getWidget", {
    name = "literal"
})

fromResource = invoke("lookup-invoke:index:getWidget", {
    name = widget.name
})

output "fromLiteral" {
    value = fromLiteral.result
}

output "fromResource" {
    value = fromResource.result
}
