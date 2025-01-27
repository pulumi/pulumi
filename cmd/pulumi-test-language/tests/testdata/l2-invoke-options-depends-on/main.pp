resource "explicitProvider" "pulumi:providers:simple-invoke" { }

resource "first" "simple-invoke:index:StringResource" {
    text = "first hello"
}

data = invoke("simple-invoke:index:myInvoke", { value = "hello" }, {
    dependsOn = [first]
})

resource "second" "simple-invoke:index:StringResource" {
    text = data.result
}

output "hello" {
    value = data.result
}
