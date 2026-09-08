resource "r" "output:index:Resource" {
    value = 1
}

# During preview `r.output` resolves to unknown. Wrapping an unknown value with
# secret() must preserve the secret marker when the stack output is serialised
# back to the engine.
output "wrapped" {
    value = secret(r.output)
}
