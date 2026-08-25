// The package name, module name, resource names, object type names and property names are all
// kebab-case.
resource "first" "kebab-names:kebab-module:some-resource" {
    the-input = true
    nested = {
        nested-value = "nested"
    }
}

resource "second" "kebab-names:kebab-module:another-resource" {
    the-input = first.the-output.nested-output
}

// Whole objects in stack outputs keep their wire-format keys
output "theOutput" {
    value = first.the-output
}

// The function name and its argument and result property names are kebab-case. The nested object
// type carries a property with a schema default value.
output "invoked" {
    value = invoke("kebab-names:kebab-module:do-something", {
        the-input = "hello"
        nested = {
            value = "nested"
        }
    }).the-output
}
