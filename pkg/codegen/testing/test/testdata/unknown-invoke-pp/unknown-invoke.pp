data = invoke("unknown:index:getData", {
    input = "hello"
})

values = invoke("unknown:eks:moduleValues", {})

output "content" {
    value = data.content
}