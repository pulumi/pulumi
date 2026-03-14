config "aNumber" "number" {}

output "theNumber" {
  value = aNumber + 1.25
}

config "optionalNumber" "number" {
  default = 41
}

output "defaultNumber" {
  value = optionalNumber + 1
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
