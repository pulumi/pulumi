resource "source" "splatoptional:index:Container" {
  inputs = ["a", "b"]
}

resource "sink" "splatoptional:index:Container" {
  inputs = source.details[*].value
}

output "values" {
  value = source.details[*].value
}
