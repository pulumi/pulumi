// Test that "pkg:typ" type tokens are accepted in PCL and are correctly expanded out. We also have an L2 test around
// this but it's worth checking with the pulumi schema as it would be too easy for codegen to special case it differently.

resource "myStash" "pulumi:Stash" {
    input = "test"
}

output "stashInput" {
    value = myStash.input
}

output "stashOutput" {
    value = myStash.output
}