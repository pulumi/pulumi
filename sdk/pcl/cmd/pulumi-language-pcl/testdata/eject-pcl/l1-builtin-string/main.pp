config "aString" "string" {}

output "lengthOutput" {
  value = length(aString)
}

output "splitOutput" {
  value = split("-", aString)
}

output "joinOutput" {
  value = join("|", split("-", aString))
}

output "interpolateOutput" {
  value = "prefix-${aString}"
}
