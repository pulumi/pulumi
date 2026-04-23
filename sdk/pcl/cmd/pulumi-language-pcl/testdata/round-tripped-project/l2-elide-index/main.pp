resource "res" "simple:Resource" {
    value = true
}

output "inv" {
    value = invoke("simple-invoke:myInvoke", {value="test"}).result
}