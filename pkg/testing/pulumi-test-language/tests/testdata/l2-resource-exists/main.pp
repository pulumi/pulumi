resource "res" "simple:index:Resource" {
    value = true
}

existsResult = resourceExists("simple:index:Resource", res.id)

output "existsResult" {
    value = existsResult
}
