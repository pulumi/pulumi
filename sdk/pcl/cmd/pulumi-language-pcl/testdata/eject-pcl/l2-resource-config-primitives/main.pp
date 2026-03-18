config "plainBool" "bool" {}
config "plainNumber" "number" {}
config "plainString" "string" {}

config "secretBool" "bool" {
  secret = true
}
config "secretNumber" "number" {
  secret = true
}
config "secretString" "string" {
  secret = true
}

resource "plain" "primitive:index:Resource" {
  boolean = plainBool
  float = plainNumber + 0.5
  integer = plainNumber
  string = plainString
  numberArray = [-1.0, 0.0, 1.0]
  booleanMap = {
    t = true
    f = false
  }
}

resource "secret" "primitive:index:Resource" {
  boolean = secretBool
  float = secretNumber + 0.5
  integer = secretNumber
  string = secretString
  numberArray = [-2.0, 0.0, 2.0]
  booleanMap = {
    t = true
    f = false
  }
}
