resource "firstResource" "keywords:index:SomeResource" {
    builtins = "builtins"
    property = "property"
}

resource "secondResource" "keywords:index:SomeResource" {
    builtins = firstResource.builtins
    property = firstResource.property
}
