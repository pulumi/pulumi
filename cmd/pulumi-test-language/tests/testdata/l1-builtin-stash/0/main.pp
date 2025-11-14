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

resource "passthroughStash" "pulumi:index:Stash" {
    input = "old"
    passthrough = true
}

output "passthroughInput" {
    value = passthroughStash.input
}

output "passthroughOutput" {
    value = passthroughStash.output
}