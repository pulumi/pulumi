resource "explicitProvider" "pulumi:providers:simple-invoke" { }

data = invoke("simple-invoke:index:myInvoke", { value = "hello" }, {
    provider = explicitProvider
    parent = explicitProvider
    version = "10.0.0"
    pluginDownloadUrl = "https://example.com/github/example"
})

output "hello" {
    value = data.result
}