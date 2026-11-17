config "numItems" "int" {}
config "itemList" "list(string)" {}

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

resource "listDynTarget" "nestedobject:index:Target" {
  options { range = itemList }
  name = "${listResource[range.key].name}!"
}
