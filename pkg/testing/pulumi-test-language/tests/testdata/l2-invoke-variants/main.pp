resource "res" "simple-invoke:index:StringResource" {
    text = "hello"
}

output "outputInput" {
    value = invoke("simple-invoke:index:myInvoke", {value=res.text}).result
}

output "unit" {
    value = invoke("simple-invoke:index:unit", {}).result
}