resource "stringOrIntegerExample1" "union:index:Example" {
  stringOrIntegerProperty = 42
}

resource "stringOrIntegerExample2" "union:index:Example" {
  stringOrIntegerProperty = "forty two"
}

resource "mapMapUnionExample" "union:index:Example" {
  mapMapUnionProperty = {
    "key1": {
      "key1a": "value1a" # ,
      # TODO this trips up Go program generator.
      # "key1b": ["a", "b", "c"]
    }
  }
}

output "mapMapUnionOutput" "any" {
  value = mapMapUnionExample.mapMapUnionProperty
}
