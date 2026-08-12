resource "source" "nestedobject:index:Container" {
  inputs = ["a", "b"]
}

resource "sink" "nestedobject:index:Container" {
  inputs = source.details[*].value
}
