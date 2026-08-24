// The package name and module name are kebab-case. Resource and object type names cannot be
// kebab-case yet (the metaschema forbids hyphens in the member segment of a token), and kebab-case
// property names are not yet handled by all code generators.
resource "first" "kebab-names:kebab-module:someResource" {
    theInput = true
    nested = {
        nestedValue = "nested"
    }
}

resource "second" "kebab-names:kebab-module:anotherResource" {
    theInput = first.theOutput.nestedOutput
}

// Whole objects in stack outputs keep their wire-format keys
output "theOutput" {
    value = first.theOutput
}
