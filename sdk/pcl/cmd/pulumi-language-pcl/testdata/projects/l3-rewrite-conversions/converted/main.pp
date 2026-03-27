config "boolean" "bool" {}
config "float" "number" {}
config "integer" "int" {}
config "string" "string" {}
config "numberArray" "list(number)" {}
config "booleanMap" "map(bool)" {}

resource "res" "primitive:index:Resource" {
    boolean = boolean
    float = float
    integer = integer
    string = string
    numberArray = numberArray
    booleanMap = booleanMap
}
