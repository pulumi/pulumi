resource "parent" "simple:index:Resource" {
    value = true
}

resource "aliasURN" "simple:index:Resource" {
    value = true
    options {
        parent = parent
        aliases = ["urn:pulumi:test::l2-resource-option-alias::simple:index:Resource::aliasURN"]
    }
}
