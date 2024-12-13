output "hello" {
    value = invoke("simple-invoke:index:myInvoke", {value="hello"}).result
}

output "goodbye" {
    value = invoke("simple-invoke:index:myInvoke", {value="goodbye"}).result
}