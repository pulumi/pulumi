config "createBool" "bool" {}

resource "boolResource" "nestedobject:index:Target" {
  options { range = createBool }
  name = "bool-resource"
}

resource "boolTarget" "nestedobject:index:Target" {
  name = "${boolResource.name}+"
}
