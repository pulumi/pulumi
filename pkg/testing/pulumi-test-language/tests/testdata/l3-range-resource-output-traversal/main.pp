resource "container" "nestedobject:index:Container" {
  inputs = ["alpha", "bravo"]
}

resource "target" "nestedobject:index:Target" {
  options {
    range = container.details
  }
  name = range.value.value
}
