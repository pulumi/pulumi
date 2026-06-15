output "both" {
    value = invoke("multi-argument-invoke:index:multiArgumentInvoke", {first = "hello", second = "world"}).result
}

output "onlyRequired" {
    value = invoke("multi-argument-invoke:index:multiArgumentInvoke", {first = "hello"}).result
}
