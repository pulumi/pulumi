// The package name, module name and property names are kebab-case. Resource and object type names
// cannot be kebab-case yet: the metaschema forbids hyphens in the member segment of a token.
resource "first" "kebab-names:kebab-module:someResource" {
    the-input = true
    nested = {
        nested-value = "nested"
    }
}

resource "second" "kebab-names:kebab-module:anotherResource" {
    the-input = first.the-output.nested-output
}

// Whole objects in stack outputs keep their wire-format keys
output "theOutput" {
    value = first.the-output
}
