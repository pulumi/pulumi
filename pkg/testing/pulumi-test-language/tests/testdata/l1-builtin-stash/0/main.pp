resource "myStash" "pulumi:index:Stash" {
    input = {
        "key": ["value", "s"]
        "": false
    }
}

output "stashInput" {
    value = myStash.input
}

output "stashOutput" {
    value = myStash.output
}