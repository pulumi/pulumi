config "names" "list(string)" {}
config "tags" "map(string)" {}

output "greetings" {
    value = [for _, name in names : "Hello, ${name}!"]
}

output "numbered" {
    value = [for i, name in names : "${i}-${name}"]
}

output "tagList" {
    value = [for k, v in tags : "${k}=${v}"]
}
