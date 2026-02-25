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
