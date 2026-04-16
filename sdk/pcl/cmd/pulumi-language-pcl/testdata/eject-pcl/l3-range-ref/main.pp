config "numItems" "int" {}
config "itemList" "list(string)" {}
config "itemMap" "map(string)" {}
config "createBool" "bool" {}

resource "numResource" "nestedobject:index:Target" {
  options { range = numItems }
  name = "num-${range.value}"
}

resource "numTarget" "nestedobject:index:Target" {
  name = "${numResource[0].name}+"
}

resource "listResource" "nestedobject:index:Target" {
  options { range = itemList }
  name = "${range.key}:${range.value}"
}

resource "listTarget" "nestedobject:index:Target" {
  name = "${listResource[1].name}+"
}

resource "mapResource" "nestedobject:index:Target" {
  options { range = itemMap }
  name = "${range.key}=${range.value}"
}

resource "mapTarget" "nestedobject:index:Target" {
  name = "${mapResource["k1"].name}+"
}

resource "boolResource" "nestedobject:index:Target" {
  options { range = createBool }
  name = "bool-resource"
}

resource "boolTarget" "nestedobject:index:Target" {
  name = "${boolResource.name}+"
}
