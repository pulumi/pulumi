config "aString" "string" {}

output "lengthResult" {
  value = length(aString)
}

output "splitResult" {
  value = split("-", aString)
}

output "joinResult" {
  value = join("|", split("-", aString))
}

output "interpolateResult" {
  value = "prefix-${aString}"
}
