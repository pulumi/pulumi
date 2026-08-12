config "aNumber" "number" {
  secret = true
}

output "roundtrip" {
  value = aNumber
}

output "theSecretNumber" {
  value = aNumber + 1.25
}
