resource "res" "simple:index:Resource" {
    value = true
}

output "name" { 
    value = pulumiResourceName(res)
}

output "type" {
    value = pulumiResourceType(res)
}