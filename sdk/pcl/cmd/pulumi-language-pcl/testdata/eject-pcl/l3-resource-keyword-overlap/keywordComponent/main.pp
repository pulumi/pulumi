config "input" "bool" {
    description = "An input passed to the component"
}

# A resource named `this` collides with the receiver pointer of the
# ComponentResource class generated for this component. NodeJS must rename the
# resource variable (e.g. to `_this`) while keeping the `parent: this` pointer
# intact.
resource "this" "simple:index:Resource" {
    value = input
}

# Referencing `this` exercises that the rename is applied to references too, not
# just the declaration.
resource "dependent" "simple:index:Resource" {
    value = this.value
}

output "result" {
    value = dependent.value
}
