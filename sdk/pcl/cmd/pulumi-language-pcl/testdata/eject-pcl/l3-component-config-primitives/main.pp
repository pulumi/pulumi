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

component "plain" "./primitiveComponent" {
  boolean = plainBool
  float = plainNumber + 0.5
  integer = plainNumber
  string = plainString
}

component "secret" "./primitiveComponent" {
  boolean = secretBool
  float = secretNumber + 0.5
  integer = secretNumber
  string = secretString
}
