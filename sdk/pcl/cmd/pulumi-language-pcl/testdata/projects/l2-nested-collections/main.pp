# A resource with deeply nested collection output properties: a list of lists of lists
# of an object type and a map of maps of maps of strings.
resource "foo" "nestedcollections:index:Foo" { }

output "secondProp" {
    value = foo.conditionSets[0][0][1].prop
}

output "leaf" {
    value = foo.privateEndpoint["outer"]["inner"]["leaf"]
}
