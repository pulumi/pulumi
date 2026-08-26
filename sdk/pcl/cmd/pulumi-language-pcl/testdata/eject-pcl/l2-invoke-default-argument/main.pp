output "result" {
    value = invoke("simple-invoke:index:invokeWithDefault", {}).result
}

output "explicitResult" {
    value = invoke("simple-invoke:index:invokeWithDefault", {
        value = "explicit"
    }).result
}
