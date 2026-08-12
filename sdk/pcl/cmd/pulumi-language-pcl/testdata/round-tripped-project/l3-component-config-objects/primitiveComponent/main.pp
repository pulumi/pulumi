config numberArray "list(number)" {}
config booleanMap "map(bool)" {}

resource "res" "primitive:index:Resource" {
  boolean = true
  float = 3.5
  integer = 3
  string = "plain"
  numberArray = numberArray
  booleanMap = booleanMap
}
