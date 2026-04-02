config "numItems" "int" {}
config "itemList" "list(string)" {}
config "itemMap" "map(string)" {}
config "createBool" "bool" {}

resource "numResource" "nestedobject:index:Target" {
  options { range = numItems }
  name = "num-${range.value}"
}

resource "listResource" "nestedobject:index:Target" {
  options { range = itemList }
  name = "${range.key}:${range.value}"
}

resource "mapResource" "nestedobject:index:Target" {
  options { range = itemMap }
  name = "${range.key}=${range.value}"
}

resource "boolResource" "nestedobject:index:Target" {
  options { range = createBool }
  name = "bool-resource"
}
