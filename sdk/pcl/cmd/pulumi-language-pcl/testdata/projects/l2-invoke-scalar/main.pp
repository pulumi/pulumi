output "scalar" {
    value = invoke("simple-invoke-with-scalar-return:index:myInvokeScalar", {value="goodbye"})
}
