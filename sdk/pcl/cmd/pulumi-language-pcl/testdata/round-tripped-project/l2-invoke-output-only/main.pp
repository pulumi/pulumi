output "hello" {
    value = invoke("output-only-invoke:index:myInvoke", {value="hello"}).result
}

output "goodbye" {
    value = invoke("output-only-invoke:index:myInvoke", {value="goodbye"}).result
}
