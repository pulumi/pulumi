resource "res" "output:index:ComplexResource" {
    value = 1
}

output "entriesOutput" {
    value = entries(res.outputMap)
}

output "lookupOutput" {
    value = lookup(res.outputMap, "x", "default")
}

output "lookupOutputDefault" {
    value = lookup(res.outputMap, "y", "default")
}

output "entriesObjectOutput" {
    value = entries(res.outputObject)
}

output "lookupObjectOutput" {
    value = lookup(res.outputObject, "output", "default")
}

output "lookupObjectOutputDefault" {
    value = lookup(res.outputObject, "missing", "default")
}
