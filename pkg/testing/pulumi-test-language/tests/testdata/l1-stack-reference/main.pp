resource "ref" "pulumi:pulumi:StackReference" {
    name = "organization/other/dev"
}

output "plain" {
    value = getOutput(ref, "plain")
}

output "secret" {
    value = getOutput(ref, "secret")
}