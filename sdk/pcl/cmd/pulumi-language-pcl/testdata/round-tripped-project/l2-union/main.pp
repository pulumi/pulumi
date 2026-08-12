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

// List<Union<String, Enum>> pattern
resource "stringEnumUnionListExample" "union:index:Example" {
  stringEnumUnionListProperty = [
    "Listen",
    "Send",
    "NotAnEnumValue",
  ]
}

// Safe enum: literal string matching an enum value
resource "safeEnumExample" "union:index:Example" {
  typedEnumProperty = "Block"
}

// Output enum: output from another resource used as enum input
resource "enumOutputExample" "union:index:EnumOutput" {
  name = "example"
}

resource "outputEnumExample" "union:index:Example" {
  typedEnumProperty = enumOutputExample.type
}
