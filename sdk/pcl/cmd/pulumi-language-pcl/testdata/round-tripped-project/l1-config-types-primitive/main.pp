config "aNumber" "number" {}

output "theNumber" {
  value = aNumber + 1.25
}

config "optionalNumber" "number" {
  default = 41.5
}

output "defaultNumber" {
  value = optionalNumber + 1.2
}

config "anInt" "int" {}

output "theInteger" { 
  value = anInt + 4
}

config "optionalInt" "int" {
  default = 1
}

output "defaultInteger" {
  value = optionalInt + 2
}

config "aString" "string" {}

output "theString" {
  value = "${aString} World"
}

config "optionalString" "string" {
  default = "defaultStringValue"
}

output "defaultString" {
  value = optionalString
}

config "aBool" "bool" {}

output "theBool" {
  value = !aBool && true
}

config "optionalBool" "bool" {
  default = false
}

output "defaultBool" {
  value = optionalBool
}
