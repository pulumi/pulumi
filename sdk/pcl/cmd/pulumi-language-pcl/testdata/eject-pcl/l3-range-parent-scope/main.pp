config "prefix" "string" {}

resource "item" "nestedobject:index:Target" {
  options { range = 2 }
  name = "${prefix}-${range.value}"
}
