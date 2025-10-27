resource "myStash" "pulumi:pulumi:Stash" {
    value = "ignored"
}

output "stashOutput" {
    value = myStash.value
}

resource "passthroughStash" "pulumi:pulumi:Stash" { 
    value = "new"
    passthrough = true
}

output "passthroughOutput" {
    value = passthroughStash.value
}