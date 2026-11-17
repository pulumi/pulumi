myInvokeResult = invoke("simple-invoke:index:myInvoke", { value = "hello" })

config "defaultFromInvoke" "string" {
    default = myInvokeResult.result
}

output "result" {
    value = defaultFromInvoke
}
