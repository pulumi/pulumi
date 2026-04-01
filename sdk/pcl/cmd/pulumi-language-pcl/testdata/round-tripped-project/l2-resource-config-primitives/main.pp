config "plainBool" "bool" {}
config "plainNumber" "number" {}
config "plainInteger" "int" {}
config "plainString" "string" {}

config "secretBool" "bool" {
  secret = true
}
config "secretNumber" "number" {
  secret = true
}
config "secretInteger" "int" {
  secret = true
}
config "secretString" "string" {
  secret = true
}

resource "plain" "primitive:index:Resource" {
  boolean = plainBool
  float = plainNumber
  integer = plainInteger
  string = plainString
  numberArray = [-1.0, 0.0, 1.0]
  booleanMap = {
    t = true
    f = false
  }
}

resource "secret" "primitive:index:Resource" {
  boolean = secretBool
  float = secretNumber
  integer = secretInteger
  string = secretString
  numberArray = [-2.0, 0.0, 2.0]
  booleanMap = {
    t = true
    f = false
  }
}
