resource "res" "large:index:Map" {
    value = "leaf"
    depth = 300
}

output "output" {
    value = res.value
}
