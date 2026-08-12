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

component "plain" "./primitiveComponent" {
  boolean = plainBool
  float = plainNumber
  integer = plainInteger
  string = plainString
}

component "secret" "./primitiveComponent" {
  boolean = secretBool
  float = secretNumber
  integer = secretInteger
  string = secretString
}
