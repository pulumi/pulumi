config "aNumber" "number" {
  secret = true
}

output "theSecretNumber" {
  value = aNumber + 1.25
}
