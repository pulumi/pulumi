resource "explicitProvider" "pulumi:providers:simple-invoke" { }

resource "first" "simple-invoke:index:StringResource" {
    text = "first hello"
}

data = invoke("simple-invoke:index:myInvoke", { value = "hello" }, {
    provider = explicitProvider
    parent = explicitProvider
    version = "10.0.0"
    pluginDownloadUrl = "https://example.com/github/example"
    dependsOn = [first]
})

resource "second" "simple-invoke:index:StringResource" {
    text = data.result
}

output "hello" {
    value = data.result
}
