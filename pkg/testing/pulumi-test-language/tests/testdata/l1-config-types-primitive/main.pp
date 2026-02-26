config "aNumber" "number" {}

output "theNumber" {
  value = aNumber + 1.25
}

config "aString" "string" {}

output "theString" {
  value = "${aString} World"
}

config "aBool" "bool" {}

output "theBool" {
  value = !aBool && true
}
