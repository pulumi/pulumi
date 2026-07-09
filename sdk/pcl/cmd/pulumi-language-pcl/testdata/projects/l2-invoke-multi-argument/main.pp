output "both" {
    value = invoke("multi-argument-invoke:index:multiArgumentInvoke", "hello", "world").result
}

output "onlyRequired" {
    value = invoke("multi-argument-invoke:index:multiArgumentInvoke", "hello").result
}
