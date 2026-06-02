resource "res" "simple:index:Resource" {
    value = true
}

existsResult = resourceExists("simple:index:Resource", "checkExists", res.id)

output "existsResult" {
    value = existsResult
}
