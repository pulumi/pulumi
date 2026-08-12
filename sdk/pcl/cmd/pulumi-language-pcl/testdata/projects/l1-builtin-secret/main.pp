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

output "double" {
  value = secret(aSecret)
}

output "open" {
  value = unsecret(aSecret)
}

output "close" {
  value = secret(notSecret)
}