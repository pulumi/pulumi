resource "res" "exists:index:Resource" {
    value = true
}

existsResult = resourceExists("exists:index:Resource", res.id)
existsWithState = resourceExists("exists:index:Resource", res.id, { value = true })
missingWithState = resourceExists("exists:index:Resource", res.id, { value = false })

output "existsResult" {
    value = existsResult
}

output "existsWithState" {
    value = existsWithState
}

output "missingWithState" {
    value = missingWithState
}
