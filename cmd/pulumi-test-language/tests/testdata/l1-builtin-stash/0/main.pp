resource "myStash" "pulumi:pulumi:Stash" {
    value = {
        "key": ["value", "s"]
        "": false
    }
}

output "stashOutput" {
    value = myStash.value
}

resource "passthroughStash" "pulumi:pulumi:Stash" { 
    value = "old"
    passthrough = true
}

output "passthroughOutput" {
    value = passthroughStash.value
}