resource "myStash" "pulumi:index:Stash" {
    input = "ignored"
}

output "stashInput" {
    value = myStash.input
}

output "stashOutput" {
    value = myStash.output
}