config "aSecret" "string" {
  secret = true
}

config "notSecret" "string" {
  secret = false
}

output "roundtripSecret" { 
  value = aSecret
}

output "roundtripNotSecret" { 
  value = notSecret
}

output "open" {
  value = unsecret(aSecret)
}

output "close" {
  value = secret(notSecret)
}