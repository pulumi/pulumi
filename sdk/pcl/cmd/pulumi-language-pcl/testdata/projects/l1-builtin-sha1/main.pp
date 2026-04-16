config "input" "string" { }

hash = sha1(input)

output "hash" {
    value = hash
}
