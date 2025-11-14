resource "myStash" "pulumi:index:Stash" {
    input = "ignored"
}

output "stashInput" {
    value = myStash.input
}

output "stashOutput" {
    value = myStash.output
}

resource "passthroughStash" "pulumi:index:Stash" {
    input = "new"
    passthrough = true
}

output "passthroughInput" {
    value = passthroughStash.input
}

output "passthroughOutput" {
    value = passthroughStash.output
}