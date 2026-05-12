config "plainBool" "bool" {}
config "plainNumber" "number" {}
config "plainInteger" "int" {}
config "plainString" "string" {}
config "plainNumericString" "string" {}

config "secretNumber" "number" {
  secret = true
}
config "secretInteger" "int" {
  secret = true
}
config "secretString" "string" {
  secret = true
}
config "secretNumericString" "string" {
  secret = true
}

component "plainValues" "./conversionComponent" {
  boolean = plainString
  float = plainInteger
  integer = plainNumericString
  string = plainNumber
}

component "secretValues" "./conversionComponent" {
  boolean = secretString
  float = secretInteger
  integer = secretNumericString
  string = secretNumber
}
