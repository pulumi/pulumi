resource "container" "nestedobject:index:Container" {
  inputs = ["alpha", "bravo"]
}

resource "mapContainer" "nestedobject:index:MapContainer" {
  tags = {
    "k1": "charlie",
    "k2": "delta",
  }
}

# A resource that ranges over a computed list
resource "listOutput" "nestedobject:index:Target" {
  options {
    range = container.details
  }
  name = range.value.value
}

# A resource that ranges over a computed map
resource "mapOutput" "nestedobject:index:Target" {
  options {
    range = mapContainer.tags
  }
  name = "${range.key}=>${range.value}"
}
