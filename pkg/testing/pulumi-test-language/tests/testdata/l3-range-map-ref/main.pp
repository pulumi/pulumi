config "itemMap" "map(string)" {}

resource "mapResource" "nestedobject:index:Target" {
  options { range = itemMap }
  name = "${range.key}=${range.value}"
}

resource "mapTarget" "nestedobject:index:Target" {
  name = "${mapResource["k1"].name}+"
}
