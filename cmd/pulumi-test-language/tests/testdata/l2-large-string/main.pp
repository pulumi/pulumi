resource "res" "large:index:String" {
    value = "hello world"
}

output "output" "string" {
    value = res.value
}