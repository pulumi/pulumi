resource "res1" "simple:index:Resource" {
    value = true
}

output "name" { 
    value = pulumiResourceName(res1)
}

output "type" {
    value = pulumiResourceType(res1)
}